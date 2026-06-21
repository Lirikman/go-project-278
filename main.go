package main

import (
	generated "code/db/generated"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jxskiss/base62"
)

// создание маршрутизатора Gin
func setupRouter() *gin.Engine {
	router := gin.Default()
	// включаем поддержку Cloudflare
	router.TrustedPlatform = gin.PlatformCloudflare
	router.ForwardedByClientIP = true
	// настраиваем доверенные прокси
	proxies := []string{"127.0.0.1", "::1"}
	err := router.SetTrustedProxies(proxies)
	if err != nil {
		log.Fatalf("error while setting up proxy")
	}
	// настройка политики разрешений
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"https://localhost:5173/"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Referer"}
	config.ExposeHeaders = []string{"Content-Range"}
	router.Use(cors.New(config))
	// подключаем монитор просмотра ошибок
	router.Use(sentrygin.New(sentrygin.Options{}))
	// подключаем инструмент восстановления сбоев
	router.Use(gin.Recovery())
	// подключаем логгер
	router.Use(gin.Logger())
	// задаём стандартный маршрут '/ping'
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	return router
}

// получение всех записей
func listLinks(db *generated.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		var paginParams generated.ListLinksParams
		// получаем параметры для пагинации
		rangeLinks := c.DefaultQuery("range", "[0,50]")
		// задаём параметры по умолчанию
		limit := 50
		offset := 0
		// задаём регулярное выражение для поиска всех чисел
		re := regexp.MustCompile(`\d+`)
		// получаем лимит записей на странице и сдвиг для вывода записей
		numRange := re.FindAllString(rangeLinks, -1)
		// проверяем корректность ввода данных
		if len(numRange) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "the range must be specified by two numbers, example: [1,4]"})
			return
		}
		idx0, _ := strconv.Atoi(numRange[0])
		idx1, _ := strconv.Atoi(numRange[1])
		// проверка на положительные значения
		if idx0 < 0 || idx1 < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "the range value must be positive"})
			return
		}
		// если первый индекс меньше второго
		if idx0 < idx1 {
			limit = idx1 - idx0
			offset = idx0
		}
		// если индексы равны
		if idx0 == idx1 {
			limit = 1
			offset = idx0
		}
		// если первый индекс больше второго
		if idx0 > idx1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "range values are specified incorrectly"})
			return
		}
		// ограничение максимального числа записей на странице
		if limit > 50 {
			limit = 50
		}
		paginParams.Limit = int32(limit)
		paginParams.Offset = int32(offset)
		links, err := db.ListLinks(c, paginParams)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "links not found"})
			return
		}
		if err != nil {
			log.Printf("database error: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		count, err := db.CounterLinks(c)
		if err != nil {
			log.Printf("get count records error: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		headerVal := fmt.Sprintf("links: %d-%d/%d", idx0, idx1, count)
		c.Header("Content-Range", headerVal)
		c.JSON(http.StatusOK, links)

	}
}

// создание новой записи
func createLink(db *generated.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		var link generated.CreateLinkParams
		// парсинг запроса
		if err := c.ShouldBindJSON(&link); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		// создаём валидатор
		validate := validator.New()
		// валидация поля original_url
		origUrl := link.OriginalUrl
		err := validate.Var(origUrl, "required,url")
		if err != nil {
			errorsMap := make(map[string]string)
			errorsMap["original_url"] = err.Error()
			c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": errorsMap})
			return
		}
		// валидация поля short_name
		shortName := link.ShortName.String
		// если поле заполнено, то проверяем валидатором
		if shortName != "" {
			err = validate.Var(shortName, "min=3,max=32")
			if err != nil {
				errorsMap := make(map[string]string)
				errorsMap["short_name"] = err.Error()
				c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": errorsMap})
				return
			}
		}

		// если короткое имя не введено, то генерируем новое имя
		if shortName == "" {
			// проверяем количество записей в БД
			count, err := db.CounterLinks(c)
			if err != nil {
				log.Printf("counter link err: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
			var lastID string
			// если БД содержит записи
			if count > 0 {
				lastRec, err := db.LastLink(c)
				if err != nil {
					log.Printf("get last link err: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
					return
				}
				// получаем текущий ID записи
				lastID = fmt.Sprintf("%d", lastRec.ID+1)
			} else {
				// иначе текущиё ID равен 1
				lastID = fmt.Sprintf("%d", 1)
			}
			// кодируем в Base62
			shortName = base62.EncodeToString([]byte(lastID))
			// если длина сгенерированного имени меньше 3 символов
			if len(shortName) < 3 {
				shortName = shortName + shortName + shortName
			}
		}
		link.ShortName = pgtype.Text{String: shortName, Valid: true}
		// проверяем имя на уникальность
		recCode, err := db.GetLinkFromCode(c, link.ShortName)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				log.Printf("get link from code error: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
		}
		emptyStruct := generated.GetLinkFromCodeRow{}
		if recCode != emptyStruct {
			errorsMap := make(map[string]string)
			errorsMap["short_name"] = "short name already in use"
			c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": errorsMap})
			return
		}
		// cоздаём запись
		res, err := db.CreateLink(c, link)
		if err != nil {
			log.Printf("create link err: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		// создаём короткое имя ссылки
		nameService := os.Getenv("SVC_NAME")
		if nameService == "" {
			nameService = "https://github.com/Lirikman/go-project-278"
		}
		shortUrl := fmt.Sprintf("%s/r/%s", nameService, shortName)
		shortUrlTxt := pgtype.Text{String: shortUrl, Valid: true}
		// добавляем короткую ссылку к записи
		var shortNameParams generated.UpdateShortNameParams
		shortNameParams.ID = res.ID
		shortNameParams.ShortUrl = shortUrlTxt
		err = db.UpdateShortName(c, shortNameParams)
		if err != nil {
			log.Printf("update short name error: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		// получаем созданную и полностью заполненную запись
		newRec, err := db.GetLink(c, res.ID)
		if err != nil {
			log.Printf("get new create link err: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusCreated, newRec)
	}
}

// обновление записи
func updateLink(db *generated.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		var updLink generated.UpdateLinkParams
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"id": "incorrect id entered"})
			return
		}
		// проверка записи в БД
		link, err := db.GetLink(c, id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "the link does not exist"})
			return
		}
		// парсинг данных
		if err := c.ShouldBindJSON(&updLink); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		// создаём валидатор
		validate := validator.New()
		// валидация поля original_url
		origUrl := updLink.OriginalUrl
		err = validate.Var(origUrl, "required,url")
		if err != nil {
			errorsMap := make(map[string]string)
			errorsMap["original_url"] = err.Error()
			c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": errorsMap})
			return
		}
		// проверка изменения поля short_name
		if link.ShortName.String != updLink.ShortName.String {
			// валидация поля short_name
			shortName := updLink.ShortName
			err = validate.Var(shortName, "required,min=3,max=32")
			if err != nil {
				errorsMap := make(map[string]string)
				errorsMap["short_name"] = err.Error()
				c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": errorsMap})
			}
			// проверяем имя на уникальность
			recCode, err := db.GetLinkFromCode(c, shortName)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					log.Printf("get link from code error: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
					return
				}
			}
			emptyStruct := generated.GetLinkFromCodeRow{}
			if recCode != emptyStruct {
				errorsMap := make(map[string]string)
				errorsMap["short_name"] = "short name already in use"
				c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": errorsMap})
				return
			}
			nameService := os.Getenv("SVC_NAME")
			if nameService == "" {
				nameService = "https://github.com/Lirikman/go-project-278"
			}
			shortUrl := fmt.Sprintf("%s/r/%s", nameService, updLink.ShortName.String)
			// изменяем короткую ссылку записи
			var shortNameParams generated.UpdateShortNameParams
			shortNameParams.ID = link.ID
			shortNameParams.ShortUrl = pgtype.Text{String: shortUrl, Valid: true}
			err = db.UpdateShortName(c, shortNameParams)
			if err != nil {
				log.Printf("update short name error: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
		}
		// обновляем остальные поля записи
		updLink.ID = id
		newLink, res := db.UpdateLink(c, updLink)
		if res != nil {
			log.Printf("update link err: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, newLink)
	}
}

// получение одной записи
func getLinkFromId(db *generated.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		link, err := db.GetLink(c, id)
		if err != nil {
			// проверяем, вызвана ли ошибка отсутствием строки в БД
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
				return
			}
			// иначе это другая ошибка сервера
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		c.JSON(http.StatusOK, link)
	}
}

// удаление записи
func deleteLink(db *generated.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// проверяем наличие записи
		_, err = db.GetLink(c, id)
		if err != nil {
			// проверяем, вызвана ли ошибка отсутствием строки в БД
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "the link does not exist"})
				return
			}
			// иначе это другая ошибка сервера
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		// удаляем ссылку
		err = db.DeleteLink(c, id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "error deleting links",
			})
			return
		}
		c.JSON(http.StatusNoContent, id)
	}
}

// перенаправление по shot_name на original_url
func redirectLink(db *generated.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		codeStr := c.Param("code")
		// проверка корректности ввода
		if codeStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "short name cannot be empty"})
			return
		}
		codeTxt := pgtype.Text{String: codeStr, Valid: true}
		// получаем id, original_url из БД по введёному имени
		codeParams, err := db.GetLinkFromCode(c, codeTxt)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "short name not found"})
			return
		}
		// добавляем запись о посещении в БД
		var visitParams generated.CreateLinkVisitsParams
		linkID := codeParams.ID
		userAgent := c.Request.UserAgent()
		ip := c.ClientIP()
		referer := c.Request.Referer()
		currentStatus := http.StatusFound
		visitParams.LinkID = linkID
		visitParams.UserAgent = pgtype.Text{String: userAgent, Valid: true}
		visitParams.Ip = pgtype.Text{String: ip, Valid: true}
		visitParams.Referer = pgtype.Text{String: referer, Valid: true}
		visitParams.Status = pgtype.Int4{Int32: int32(currentStatus), Valid: true}
		_, err = db.CreateLinkVisits(c, visitParams)
		if err != nil {
			log.Printf("create link visits error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		// перенапраявляем на оригинальный адрес
		c.Redirect(http.StatusFound, codeParams.OriginalUrl)
	}
}

// получение статистики
func listVisits(db *generated.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		var paginParams generated.ListLinkVisitsParams
		// получаем параметры для пагинации
		rangeLinks := c.DefaultQuery("range", "[0,50]")
		// задаём параметры по умолчанию
		limit := 50
		offset := 0
		// задаём регулярное выражение для поиска всех чисел
		re := regexp.MustCompile(`\d+`)
		// получаем лимит записей на странице и сдвиг для вывода записей
		numRange := re.FindAllString(rangeLinks, -1)
		// проверяем корректность ввода данных
		if len(numRange) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "the range must be specified by two numbers, example: [1,4]"})
			return
		}
		idx0, _ := strconv.Atoi(numRange[0])
		idx1, _ := strconv.Atoi(numRange[1])
		// проверка на положительные значения
		if idx0 < 0 || idx1 < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "the range value must be positive"})
			return
		}
		// если первый индекс меньше второго
		if idx0 < idx1 {
			limit = idx1 - idx0
			offset = idx0
		}
		// если индексы равны
		if idx0 == idx1 {
			limit = 1
			offset = idx0
		}
		// если первый индекс больше второго
		if idx0 > idx1 {
			msg := "range values are specified incorrectly"
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		// ограничение максимального числа записей на странице
		if limit > 50 {
			limit = 50
		}
		paginParams.Limit = int32(limit)
		paginParams.Offset = int32(offset)
		// получаем все записи из БД
		links, err := db.ListLinkVisits(c, paginParams)
		if err == sql.ErrNoRows {
			log.Printf("list link visits error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if err != nil {
			log.Printf("get link visits error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		count, err := db.CounterVisits(c)
		if err != nil {
			log.Printf("get counter visits error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		headerVal := fmt.Sprintf("link_visits: %d-%d/%d", idx0, idx1, count)
		c.Header("Content-Range", headerVal)
		c.JSON(http.StatusOK, links)
	}
}

func main() {
	// подулючаемся к БД
	var err error
	// Инициализация пула соединений
	conn, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка подключения к БД: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	queries := generated.New(conn)

	// подключаем мониторинг ошибок
	errSentry := sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
	})
	if errSentry != nil {
		log.Fatalf("sentry initialization failed: %s", errSentry)
	}
	defer sentry.Flush(2 * time.Second)

	// создаём маршрутизатор
	r := setupRouter()

	// регистрируем маршруты
	r.GET("/api/links", listLinks(queries))
	r.GET("/api/links/:id", getLinkFromId(queries))
	r.GET("/api/link_visits", listVisits(queries))
	r.GET("/r/:code", redirectLink(queries))
	r.POST("/api/links", createLink(queries))
	r.PUT("/api/links/:id", updateLink(queries))
	r.DELETE("/api/links/:id", deleteLink(queries))

	// запускаем сервер на порту 8080
	port := os.Getenv("SVC_PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server startup error")
	}
}
