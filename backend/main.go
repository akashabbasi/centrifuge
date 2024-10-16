package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	pgadapter "github.com/casbin/casbin-pg-adapter"
	"github.com/casbin/casbin/v2"
	"github.com/golang-jwt/jwt/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/bcrypt"
)

// User model
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// JWT secret key
var jwtSecret = []byte("secret")

// Database connection
var db *sql.DB

// Casbin Enforcer
var enforcer *casbin.Enforcer

// JWT claims
type JwtCustomClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Initialize the database connection
func initDB() {
	var err error
	dsn := "postgres://auth_user:password@db:5432/auth_service?sslmode=disable"
	db, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal("Failed to ping the database:", err)
	}
}

// Initialize Casbin with PostgreSQL adapter
func initCasbin() {
	adapter, err := pgadapter.NewAdapter("postgresql://auth_user:password@db:5432/auth_service?sslmode=disable")
	if err != nil {
		log.Fatal("Failed to create Casbin adapter:", err)
	}

	enforcer, err = casbin.NewEnforcer("casbin/casbin_model.conf", adapter)
	if err != nil {
		log.Fatal("Failed to create Casbin enforcer:", err)
	}

	// Load Casbin policies from DB
	err = enforcer.LoadPolicy()
	if err != nil {
		log.Fatal("Failed to load Casbin policies:", err)
	}
}

// Signup handler
func signup(c echo.Context) error {
	user := new(User)
	if err := c.Bind(user); err != nil {
		return err
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Insert user into database
	_, err = db.Exec("INSERT INTO users (username, password) VALUES ($1, $2)", user.Username, string(hashedPassword))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, map[string]string{"message": "User created successfully"})
}

// Signin handler
func signin(c echo.Context) error {
	user := new(User)
	if err := c.Bind(user); err != nil {
		return err
	}

	// Fetch user from DB
	var storedPassword string
	var userID int
	err := db.QueryRow("SELECT id, password FROM users WHERE username=$1", user.Username).Scan(
		&userID,
		&storedPassword)
	if err != nil {
		return echo.ErrUnauthorized
	}

	// Compare password
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(user.Password))
	if err != nil {
		return echo.ErrUnauthorized
	}

	// Create JWT
	// claims := &JwtCustomClaims{
	// 	Username: user.Username,
	// 	RegisteredClaims: jwt.RegisteredClaims{
	// 		Subject:   strconv.Itoa(userID),
	// 		IssuedAt:  jwt.NewNumericDate(time.Now()),
	// 		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	// 	},
	// }
	claims := jwt.MapClaims{
		"sub":      strconv.Itoa(userID),             // Subject (user ID)
		"exp":      time.Now().Add(time.Hour).Unix(), // Expiration time (1 hour from now)
		"iat":      time.Now().Unix(),                // Issued at time
		"username": user.Username,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString(jwtSecret)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{"token": t})
}

// Define the struct for the request body
type RequestBody struct {
	Client  string `json:"client"`
	Channel string `json:"channel"`
}

// Subscription proxy for Centrifugo
func subscriptionProxy(c echo.Context) error {
	authHeader := c.Request().Header.Get("Authorization")
	// fmt.Println("Auth Header:: ", c.Request().Body)
	// Initialize the struct
	var body RequestBody

	// Bind the request body to the struct
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid request")
	}

	// Print the request body struct
	fmt.Printf("Received body: %+v\n", body)
	if authHeader == "" || len(authHeader) < 7 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing or invalid Authorization header")
	}
	tokenStr := authHeader[7:]

	// Validate token
	claims := &JwtCustomClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
	}

	// Casbin check
	// user := claims.Username
	channel := c.QueryParam("channel")
	// action := c.QueryParam("action")

	// ok, err := enforcer.Enforce(user, channel, action)
	// if err != nil || !ok {
	// 	return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	// }

	return c.JSON(http.StatusOK, map[string]interface{}{"result": map[string]interface{}{"channels": []string{channel}}})
}

func main() {
	// Initialize database and Casbin
	initDB()
	initCasbin()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.POST("/signup", signup)
	e.POST("/signin", signin)
	e.POST("/subscribe", subscriptionProxy)

	e.Logger.Fatal(e.Start(":9080"))
}
