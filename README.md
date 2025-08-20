# Chirpy

A Twitter-like social media REST API built with Go that allows users to post short messages called "chirps".

## 🚀 Features

- **User Management**: Register, login, and update user accounts
- **Authentication**: JWT-based authentication with refresh tokens
- **Chirps**: Create, read, and delete short messages (max 140 characters)
- **Content Moderation**: Automatic profanity filtering
- **Sorting & Filtering**: Sort chirps by creation date, filter by author
- **Premium Features**: Chirpy Red subscription upgrades via webhook
- **Admin Dashboard**: Visit metrics and development tools

## 🛠️ Tech Stack

- **Language**: Go 1.24
- **Database**: PostgreSQL with SQLC for type-safe queries
- **Authentication**: JWT tokens with refresh token rotation
- **Environment**: Environment variables via godotenv
- **HTTP**: Go's native `net/http` package

## 📋 Prerequisites

- Go 1.24 or higher
- PostgreSQL database
- Environment variables configured (see Configuration section)

## ⚙️ Configuration

Create a `.env` file with the following variables:
```
env DB_URL=postgres://username:password@localhost/chirpy?sslmode=disable 
PLATFORM=dev 
SVR_SECRET=your-jwt-secret-key 
POLKA_KEY=your-polka-api-key

```