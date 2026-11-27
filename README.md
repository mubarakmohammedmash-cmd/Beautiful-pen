# The Beautiful Pen Integrated Services Limited

## Project Structure

- `frontend/` — HTML website, favicon, robots.txt and sitemap.xml
- `backend/` — Go HTTP server, API, SMTP email delivery and `.env.example`
- `backend/data/` — private enquiry submissions (`submissions.jsonl`)

## Run Locally

```sh
cd backend
go run main.go
```

Open: http://localhost:8080

Health check: http://localhost:8080/api/health

## Production Build

```sh
go build -o backend/server ./backend
```

Run:

```sh
./backend/server
```

## Email Setup

Copy `backend/.env.example` values into environment variables. Gmail requires an App Password.

Required variables:

- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USER`
- `SMTP_PASS`
- `TARGET_EMAIL` (optional; defaults to the company email)

## API

`POST /api/contact`

```json
{
  "name": "Customer Name",
  "phone": "08000000000",
  "email": "customer@example.com",
  "subject": "Residential Home Build",
  "message": "Project details"
}
```
