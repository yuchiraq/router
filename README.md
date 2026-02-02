# Go Reverse Proxy with Admin Panel

This project is a web server written in Go that functions as a reverse proxy. It includes an admin panel for managing routing rules, viewing statistics, and streaming logs in real time.

## Features

- **Reverse Proxy:** Routes incoming HTTP and HTTPS requests to various backend services based on customizable rules.
- **Admin Panel:** A web interface for:
  - Adding and removing routing rules.
  - Viewing real-time server statistics (e.g., memory usage, request count).
  - Streaming server logs via WebSockets.
- **Automatic HTTPS:** Uses `autocert` to automatically obtain and renew TLS certificates from Let's Encrypt.
- **Security:** The admin panel is protected by basic authentication.

## Getting Started

### Prerequisites

- Go 1.24.0 or newer.
- A registered domain name.

### Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/your-username/your-repo-name.git
   cd your-repo-name
   ```

2. **Set environment variables:**
   Create a `.env` file in the project root and add the following:
   ```
   ADMIN_USER=your_admin_username
   ADMIN_PASS=your_admin_password
   ```

3. **Run the server:**
   ```bash
   go run main.go
   ```

## Usage

- **Proxy:** The proxy server runs on ports 80 (HTTP) and 443 (HTTPS).
- **Admin Panel:** The admin panel is available at `http://localhost:8162`.

## Project Structure

- `main.go`: The main entry point of the application.
- `cmd/main.go`: An alternative main entry point.
- `internal/`: Contains the application's internal packages.
  - `logstream/`: Log streaming via WebSocket.
  - `panel/`: Admin panel handlers and templates.
  - `proxy/`: Reverse proxy logic.
  - `stats/`: Statistics collection.
  - `storage/`: Routing rule storage.
- `go.mod`, `go.sum`: Go module files.
- `GEMINI.md`: AI rules for the project.
