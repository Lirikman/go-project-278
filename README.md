# URL Shortener


## Hexlet tests and linter status:
[![Actions Status](https://github.com/Lirikman/go-project-278/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/Lirikman/go-project-278/actions)

## Description

Designed to shorten long links to shorter ones, saving them in the database
The service has a graphical interface and API.

## Service address:
https://go-project-278-bxbz.onrender.com

## Requirements
The service has a validator to check the correctness of the data entered
* original URL address must be correct
* short name must be unique
* length of a short name is from 3 to 32 characters

## API request examples

### Getting a list of links
Returns a list of all saved links
The maximum number of records displayed on one page is 50.
You can also get a range of links
The range boundaries must be set correctly, example range=[1,6]

**GET** /api/links

**Example answer:**
```json
[
  {
    "id": 1,
    "original_url": "https://example.com/long-url",
    "short_name": "exmpl",
    "short_url": "https://short.io/r/exmpl"
  },
  {
    "id": 2,
    "original_url": "https://example.com/long-url2",
    "short_name": "exmpl",
    "short_url": "https://short.io/r/exmpl2"
  }
]
```
Response code: 200 OK

**GET** /api/links?range=[1,3]

**Example answer:**
```json
[
  {
    "id": 2,
    "original_url": "https://www.youtube.com/watch?v=zJNkxuechzY",
    "short_name": "gimnast",
    "short_url": "https://go-project-278-yoao.onrender.com/r/gimnast"
  },
  {
    "id": 4,
    "original_url": "https://overclockers.ru/blog/cool-gadgets/show/226371/Pochemu-net-smysla-pokupat-noutbuk-s-GeForce-RTX-5060-i-kogda-igrovye-leptopy-v-RF-podeshevejut",
    "short_name": "gtx4060",
    "short_url": "https://go-project-278-yoao.onrender.com/r/gtx4060"
  }
]
```
Response code: 200 OK
Content-Range: links 1-3/10

### Creating a new link
Creates a new link in the database. 
If a short name is not entered, the service generates one automatically.

**POST** /api/links

Request body:
{
  "original_url": "https://example.com/main/auto/long-url",
  "short_name": "exmpl"
}

**Example answer:**
```json
{
  "id": 5,
  "original_url": "https://example.com/main/auto/long-url",
  "short_name": "exmpl",
  "short_url": "https://go-project-278-yoao.onrender.com/r/exmpl"
}
```
Response code: 201 Created

### Getting a link by ID
Returns a link by ID or an error that the link is not found

**GET** api/links/3

**Example answer:**
```json
{
  "id": 3,
  "original_url": "https://go-cookbook.com/snippets/cli-tools/building-cli-applications",
  "short_name": "golang",
  "short_url": "https://go-project-278-yoao.onrender.com/r/golang"
}
```
Response code: 200 OK

**GET** api/links/55

**Example answer:**
```json
{"error":"link not found"}
```
Response code: 404 NotFound

### Updating an existing link
Change of short name and original address URL by ID

**PUT** /api/links/:id

Request body:
{
  "original_url": "https://example.com/long-url2",
  "short_name": "exmpl2"
}

**Example answer:**
```json
{
  "id": 1,
  "original_url": "https://example.com/long-url2",
  "short_name": "exmpl2",
  "short_url": "https://short.io/r/exmpl2"
}
```
Response code: 200 OK

### Removing a link
Removes a link from the database by ID

**DELETE** /api/links/3
Response code: 204 No Content

### Adding a visit record and redirect
Adds a visit record to the database

**GET** /r/exmpl2
Response code: 302 Found

### Getting a list of visits
Returns all site visit records by short name
The maximum number of records displayed on one page is 50.
You can also obtain a number of visit records
The range boundaries must be set correctly, example range=[1,6]

**GET** /api/link_visits

**Example answer:**
```json
[
  {
    "id": 1,
    "link_id": 10,
    "created_at": "2026-05-26T13:12:09.626471Z",
    "ip": "176.215.124.171",
    "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
    "status": 302
  },
  {
    "id": 2,
    "link_id": 4,
    "created_at": "2026-05-26T13:12:40.500486Z",
    "ip": "176.215.124.171",
    "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
    "status": 302
  }
 ]
```
Response code: 200 OK

**GET** /api/link_visits?range=[1,4]

**Example answer:**
```json
[
  {
    "id": 2,
    "link_id": 4,
    "created_at": "2026-05-26T13:12:40.500486Z",
    "ip": "176.215.124.171",
    "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
    "status": 302
  },
  {
    "id": 3,
    "link_id": 11,
    "created_at": "2026-05-26T13:13:35.196742Z",
    "ip": "176.215.124.171",
    "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
    "status": 302
  }
]
```

