# API Guide

This document provides details on the publicly accessible APIs for the dafuweng microservices.

## Login Server API

The Login Server handles user authentication and session management. It exposes an HTTP/JSON API.

Base URL: (Assuming loginserver runs on `http://localhost:8081` as per `cmd/loginserver/loginserver.go` default)

### 1. User Login

*   **Endpoint:** `/api/login`
*   **Method:** `POST`
*   **Description:** Authenticates a user based on username and password. If authentication is successful and the user does not exist, a new user account is created. A session token is returned upon successful login.
*   **Request Body (JSON):**
    Corresponds to `pb.LoginRequest`.
    ```json
    {
      "username": "user1",
      "password": "password123"
    }
    ```
    *   `username` (string, required): The user's chosen username.
    *   `password` (string, required): The user's password. (Note: Currently, any non-empty password is accepted for a non-empty username).
*   **Response Body (JSON on Success - HTTP 200 OK):**
    Corresponds to `pb.LoginResponse`.
    ```json
    {
      "user_id": "generated_or_existing_uuid",
      "nickname": "user1", 
      "session_token": "generated_session_token_string",
      "success": true,
      "error_message": "" 
    }
    ```
    *   `user_id`: Unique identifier for the user.
    *   `nickname`: User's nickname (currently same as username).
    *   `session_token`: Token to be used for subsequent authenticated requests. Sessions are stored in Redis and expire after 24 hours.
    *   `success`: Always `true` on successful login.
*   **Response Body (JSON on Failure):**
    Corresponds to `pb.LoginResponse` with `success: false`.
    ```json
    {
      "user_id": "",
      "nickname": "",
      "session_token": "",
      "success": false,
      "error_message": "Description of the error (e.g., 'Username and password are required', 'Database error while finding player.', 'Failed to store session for new player.')"
    }
    ```
    *   **HTTP 400 Bad Request:** If `username` or `password` are empty.
    *   **HTTP 500 Internal Server Error:** For database issues or problems storing the session in Redis.

### 2. Validate Session

*   **Endpoint:** `/api/validate_session`
*   **Method:** `POST`
*   **Description:** Validates an existing session token.
*   **Request Body (JSON):**
    Corresponds to `pb.ValidateSessionRequest`.
    ```json
    {
      "session_token": "an_existing_session_token"
    }
    ```
    *   `session_token` (string, required): The session token to validate.
*   **Response Body (JSON on Success - HTTP 200 OK, token is valid):**
    Corresponds to `pb.ValidateSessionResponse`.
    ```json
    {
      "user_id": "associated_user_uuid",
      "is_valid": true,
      "error_message": ""
    }
    ```
    *   `user_id`: The unique identifier of the user associated with the valid session.
    *   `is_valid`: Always `true` if the token is valid.
*   **Response Body (JSON on Failure - HTTP 401 Unauthorized, token is invalid/expired/not found):**
    Corresponds to `pb.ValidateSessionResponse` with `is_valid: false`.
    ```json
    {
      "user_id": "",
      "is_valid": false,
      "error_message": "session not found or expired" 
    }
    ```
*   **Response Body (JSON on Bad Request - HTTP 400 Bad Request):**
    If the request format is invalid (e.g., malformed JSON).
    ```json
    {
      "user_id": "",
      "is_valid": false,
      "error_message": "Invalid request format: ..."
    }
    ```

### Session Management Notes:
*   Sessions are stored in Redis.
*   Each session token is associated with a `user_id`.
*   Sessions currently have a Time-To-Live (TTL) of 24 hours from the time of creation (login).
*   There is no session refresh mechanism implemented in `ValidateSession` at this time.
---

*Further APIs for other services like `gameserver` (if it exposes HTTP), `roomserver` (RPC details), etc., would be documented here or in separate files as the project grows.*
