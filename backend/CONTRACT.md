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

# Accounts
## Update Account
Endpoint: `PATCH /api/accounts/{id}`

Requires the `session` cookie. Each session is mapped to exactly one user

Expected Input:
```json
{
    "name": string,                 // OPTIONAL
    "currency": string,             // OPTIONAL
    "balanceInMinorUnits": int64    // OPTIONAL
}
```

Validate with the same rules as account creation

Expected Return 
```json
{
    "data": {
        "id":   string,
        "name": string,                 
        "currency": string,             
        "balanceInMinorUnits": int64,
    },
    "errors": null,
}
```

Updates the user's bank account

Omitted or null fields are left unchanged

An empty body simply returns the account and changes nothing

If the `id` is not a valid uuid, return 400 (`VALIDATION_ERROR`, "field": "id")

To make balance changes smoother, if `currency` is present, then `balanceInMinorUnits` is required
Otherwise, return 400 (`VALIDATION_ERROR`, "field": "balanceInMinorUnits")

If the account does not exist within the user's accounts, return 404 (`NOT_FOUND`)
404 also covers accounts owned by other users

### Response Codes
200 - Resource was successfully modified
400 - Validation Error
401 - User not authenticated
404 - Not Found
409 - An account name already exists within the user's accounts

## Delete Account
Endpoint: `DELETE /api/accounts/{id}`

Requires the `session` cookie. Each session is mapped to exactly one user

Expected Return 
```json
{
    "data": {
        "id":   string,
        "name": string,                 
        "currency": string,             
        "balanceInMinorUnits": int64,
    },
    "errors": null,
}
```

Deletes a user's savings account

Account deletions are hard deletes

If `id` does not exist within the user's data, return 404

If the `id` is an invalid uuid4 string, return 400 (`VALIDATION_ERROR`)

### Response Codes
200 - Successful Deletion
400 - Bad request (`VALIDATION_ERROR`)
401 - User not authenticated (`AUTHENTICATION_ERROR`)

## Create Account
Endpoint: `POST /api/accounts`

Requires the `session` cookie. Each session is mapped to exactly one user

Expected Inputs:
```js
{
    "name": string,             // REQUIRED
    "currency": string,         // REQUIRED
    "balanceInMinorUnits": int64, // OPTIONAL: Default to 0
}
```

Expected Return: 
```json
{
    "data": {
        "id":   string,
        "name": string,                 
        "currency": string,             
        "balanceInMinorUnits": int64,
    },
    "errors": null,
}
```

Creates an account for the user

Currency is in ISO 4217 code such as the following below
Example:
- PHP - Philippine Peso
- USD - US Dollar
- EUR - Euro

`balanceInMinorUnits` is an integer in the currency's smallest unit, using that currency's ISO 4217 decimal places (exponent):

| currency | exponent | balance |     means   |
|----------|----------|---------|-------------|
|    PHP   |    2     |  1050   |  10.50 PHP  |
|    USD   |    2     |  1050   |  10.50 USD  |
|    JPY   |    0     |  1050   |  1050 JPY   |

Client will convert the balance with `amount = balanceInMinorUnits / 10^exponent`

Invalid codes return 400 (VALIDATION_ERROR, field: "currency")

Negative balances return 400 (VALIDATION_ERROR, field: "balanceInMinorUnits")

Empty names return 400 (VALIDATION_ERROR, field: "name")

Names should be unique for each user. Account names are case sensitive.
Example:
- User Roman can have an account named "PayPal" and Talia can also have an account named "PayPal"
- User Benjamin cannot have two accounts named "PayPal"
- However, Benjamin can have two accounts named "PayPal" and "paypal"

### Response Codes
201 - Created
400 - Bad request (`VALIDATION_ERROR`)
401 - User not authenticated (`AUTHENTICATION_ERROR`)
409 - Account name already exists (`CONFLICT`)

## Get Accounts 
Endpoint: `GET /api/accounts`

Requires the `session` cookie. Each session is mapped to exactly one user

Expected Return:
```json
{
    "data": {
        "accounts": [
            {
                "id":                   string,
                "name":                 string,
                "currency":             string, 
                "balanceInMinorUnits":  int64,
            },
        ]
    },
    "errors": null,
}
```

Returns the user's accounts
### Response Codes
200 - OK
401 - User not authenticated (`AUTHENTICATION_ERROR`)

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
