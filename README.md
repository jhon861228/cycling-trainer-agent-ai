# Cycling Trainer Agent AI

AI-powered cycling coach.

## Local Development

### 1. Infrastructure (DynamoDB Local)
Start DynamoDB local using Docker:
```bash
docker-compose up -d
```

Initialize the local table:
```bash
chmod +x scripts/init-local-db.sh
./scripts/init-local-db.sh
```

### 2. Backend (Go)
Set environment variables and run:
```bash
export DYNAMODB_ENDPOINT=http://localhost:8001
export DYNAMODB_TABLE=CyclingTrainerData
export GEMINI_API_KEY=your_key_here
cd backend

# Option A: Manual run
go run main.go

# Option B: Hot Reload (Automatic restart)
# First install air: go install github.com/air-verse/air@latest
air
```

### 3. Frontend (Astro)
Install dependencies and run:
```bash
cd frontend
npm install
npm run dev
```
Update `API_BASE` in components to `http://localhost:8080`.
