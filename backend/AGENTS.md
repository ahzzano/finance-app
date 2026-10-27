This is the backend of the budgeting app

# Structure
`cmd/` - Initializes the API. 
`internal/`  
    - `database/` - Database related items 
    - `jobs/` - Background jobs. 
`middleware/` - Middleware
`services/` - Where most of the business logic lies in
`tests/` - Integration tests. Prefer using Go's first-party libraries instead of third-party libraries when making n
`main.go` - Starting point of the backend
`CONTRACT.md` - The expected endpoints that the backend SHOULD have as well as expected response codes
`sqlc.yaml` - `sqlc` configuration. Do not touch

# Guidelines
- The backend must always be stateless. The backend server should not store state related to the user
    - All information related to the user must always be stored in a database
- Use Session Cookies for storing user sessions. Avoid using JWTs
- Always store credentials (Database URLs, login details, etc.) in a .env file
- Prefer integration tests over unit tests unless absolutely necessary (i.e. testing a data structure implementation). 
- Assume that you cannot trust the client connecting to you. Even if it is the official frontend, assume otherwise
- Avoid blowing up `main.go` with code. Keep it as lean as possible
- Document API using swaggo

## CONTRACT.md Rules
`CONTRACT.md` contains the overall specification of the backend. As such, it is one of the most important files when
developing the backend
- If `CONTRACT.md` does not exist, DO NOT MODIFY THE BACKEND CODEBASE. Under no circumstance should you proceed when the API Contracts are not yet created
- You can only read from `CONTRACT.md`. Do not modify it unless the user specifically allows you to
- Under no circumstance should you delete `CONTRACT.md`

If the user allows you to edit `CONTRACT.md`, the format is as follows

```markdown
# Service Name
## Feature Name 
Endpoint: `METHOD /api/endpoint`

Expected Return: 
{
    "data": {
        "insert_data_here": "insert_value_here
    },
    "errors": null
}

Description of what it does

### Response Codes
200 - OK
403 - Forbidden
```
