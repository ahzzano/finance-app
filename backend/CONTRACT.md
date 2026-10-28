# Conventions
Applies to every endpoint below.

## Response Envelope
Every response is `{"data": ..., "errors": ...}`. On success `errors` is `null`; on failure `data` is `null`.

## Error Format
```json
{
    "data": null,
    "errors": [
        {
            "type": "VALIDATION_ERROR",
            "message": "email is required",
            "field": "email"
        }
    ]
}
```

`field` is optional and omitted when the error is not tied to a specific input field.

Allowed `type` values:
| type | Status code |
|------|-------------|
| `VALIDATION_ERROR` | 400 |
| `AUTHENTICATION_ERROR` | 401 |
| `NOT_FOUND` | 404 |
| `METHOD_NOT_ALLOWED` | 405 |
| `CONFLICT` | 409 |
| `RATE_LIMITED` | 429 |
| `INTERNAL_ERROR` | 500 |

## Session
The session is carried only by the `session` cookie (`HttpOnly; Secure; SameSite=Lax; Path=/`, `Max-Age` = server session TTL).
Session tokens are never returned in response bodies. Authenticated endpoints read the session solely from this cookie.
A missing, invalid, or expired session returns `401` (`AUTHENTICATION_ERROR`).

## Common Response Codes
500 - Internal server error (`INTERNAL_ERROR`). Applies to every endpoint.

# User
Service handles all user related features

## Logout
Endpoint: `POST /api/users/logout`

Requires the `session` cookie. Each session is mapped to exactly one user

Expected Return:
```json
{
    "data": {
        "message": "user is successfully logged out"
    },
    "errors": null
}
```

Logs the user out of the backend. The server invalidates the session and clears the `session` cookie (`Max-Age=0`)

### Response Codes
200 - OK
401 - User was not logged in or the session is invalid (`AUTHENTICATION_ERROR`)

## Profile
Endpoint: `GET /api/users/profile`

Requires the `session` cookie. Each session is mapped to exactly one user

Expected Return:
```json
{
    "data": {
        "name": string,
        "email": string,
    },
    "errors": null
}
```

Returns a User's profile. The `session` cookie authenticates that the client is the profile owner.
Once verified, return the user's profile details.

### Response Codes
200 - OK
401 - User was not logged in or the session is invalid (`AUTHENTICATION_ERROR`)

## Login
Endpoint: `POST /api/users/login`

Expected Inputs:
```js
{
    "email": string,
    "password": string
}
```

Expected Return:
```json
{
    "data": {
        "name": string
    },
    "errors": null
}
```

Logs a user in and sets the `session` cookie. The session token is not returned in the body

### Response Codes
200 - OK
400 - Bad request (`VALIDATION_ERROR`)
401 - Login failed due to bad credentials (`AUTHENTICATION_ERROR`)
429 - Too many failed login attempts, try again later (`RATE_LIMITED`)

## Registration
Endpoint: `POST /api/users/register`

Expected Inputs:
```js
{
    "email": string,
    "password": string,
    "name": string
}
```

Expected Return:
```json
{
    "data": {
        "name": string
    },
    "errors": null
}
```

Registers a user and sets the `session` cookie. The session token is not returned in the body.
Note that each email must be unique. Registered emails cannot be used for multiple accounts

### Response Codes
201 - Created
400 - Bad request (`VALIDATION_ERROR`)
409 - Email already exists in the system (`CONFLICT`)
