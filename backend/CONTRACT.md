# User
Service handles all user related features
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
        "token": "token"
    },
    "errors": null
}
```

Logins a user and returns a session token.
The endpoint must set the cookie to the token as well

### Response Codes
200 - OK
400 - Bad request
401 - Login failed due to bad credentials
403 - Login was blocked

## Registration
Endpoint: `PUT /api/users/register`
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
        "token": "token"
    },
    "errors": null
}
```

Registers a user and returns a session token. Note that each email must be unique. Registered emails cannot be used for multiple accounts
The endpoint must set the cookie to the token as well
### Response Codes
200 - OK
400 - Bad request 
409 - Email already exists in the system
