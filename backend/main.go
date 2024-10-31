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
	"github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

// JWT secret key (replace with your own secret)
var jwtSecret = []byte("secret")

// Casbin enforcer instance
var enforcer *casbin.Enforcer

// User struct for login/signup
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// SubscriptionRequest represents the Centrifugo subscription proxy request
type SubscriptionRequest struct {
	ClientID  string `json:"client"`
	Channel   string `json:"channel"`
	Token     string `json:"token"`
	Transport string `json:"transport"`
	Protocol  string `json:"protocol"`
	Encoding  string `json:"encoding"`
	User      string `json:"user"`
}

// Initialize Casbin enforcer with PostgreSQL adapter
func InitEnforcer(db *sql.DB) (*casbin.Enforcer, error) {
	adapter, err := pgadapter.NewAdapter("postgresql://auth_user:password@db:5432/auth_service?sslmode=disable", "auth_service")
	if err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewEnforcer("casbin/casbin_model.conf", adapter)
	if err != nil {
		return nil, err
	}

	err = enforcer.LoadPolicy()
	if err != nil {
		return nil, err
	}

	policies, _ := enforcer.GetPolicy()
	fmt.Println("Loaded policies:", policies)

	return enforcer, nil
}

// CreateJWT generates a JWT token with user ID and role claims
func CreateJWT(userID int, role string) (string, error) {
	// personalChannel := fmt.Sprintf("personal:user#%d", userID)
	claims := jwt.MapClaims{
		// "sub":  fmt.Sprintf("%s:%d", role, userID),
		"sub":  strconv.Itoa(userID),
		"role": role,
		"exp":  time.Now().Add(time.Hour * 72).Unix(),

		// "channels": []string{personalChannel},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Signup handler to register a new user
func SignupHandler(c echo.Context) error {
	user := new(User)

	if err := c.Bind(user); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request payload"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Error hashing password"})
	}

	_, err = db.Exec("INSERT INTO users (username, password) VALUES ($1, $2)", user.Username, string(hashedPassword))
	fmt.Println(err)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Error creating user"})
	}

	return c.JSON(http.StatusOK, echo.Map{"message": "User registered successfully!"})
}

// Login handler to authenticate user and generate JWT token
func LoginHandler(c echo.Context) error {
	loginRequest := new(User)

	if err := c.Bind(loginRequest); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request payload"})
	}

	var user User
	err := db.QueryRow("SELECT id, password, role FROM users WHERE username = $1", loginRequest.Username).Scan(&user.ID, &user.Password, &user.Role)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Invalid username or password"})
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginRequest.Password))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Invalid username or password"})
	}

	token, err := CreateJWT(user.ID, user.Role)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Error generating token"})
	}

	return c.JSON(http.StatusOK, echo.Map{"token": token})
}

// Subscription handler for Centrifugo to authorize subscriptions
func SubscriptionHandler(c echo.Context) error {
	// Retrieve all headers from the request
	headers := c.Request().Header

	// Iterate over the headers and print them
	for key, values := range headers {
		// Header can have multiple values, so iterate over them
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}

	// authHeader := c.Request().Header.Get("Authorization")
	// fmt.Println("Req Body:: ", c.Request().Body)
	// fmt.Println("Auth Header:: ", authHeader)
	// Initialize the struct
	var body SubscriptionRequest

	// Bind the request body to the struct
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid request")
	}

	// Print the request body struct
	fmt.Printf("Received body: %+v\n", body)
	// userRoleID := body.User
	// result := strings.Split(userRoleID, ":")
	// fmt.Println("result: ", result)
	// role := result[0]

	// Casbin check
	// user := claims.Username
	// channel := c.QueryParam("channel")
	// action := c.QueryParam("action")
	// fmt.Println("role, channel", role, channel)
	// fmt.Println("input:: ", result[1], body.Channel, "subscribe")
	// Fetch all roles for the user
	err := enforcer.LoadPolicy()
	if err != nil {
		log.Fatalf("Failed to load policies: %v", err)
	}

	roles, err := enforcer.GetRolesForUser("1")
	if err != nil {
		log.Fatalf("Failed to get roles for user: %v", err)
	}

	fmt.Printf("Roles for user %s: %v\n", body.User, roles)
	ok, err := enforcer.Enforce(body.User, body.Channel, "subscribe")
	fmt.Println("ok, err::>>", ok, err)
	if err != nil || !ok {
		// Return permission denied response
		return c.JSON(http.StatusOK, echo.Map{
			"error": echo.Map{
				"code":    http.StatusForbidden,
				"message": "permission denied",
			},
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"result": map[string]interface{}{}})
}

func main() {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		"db", 5432, "auth_user", "password", "auth_service")
	// Connect to PostgreSQL
	var err error
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		fmt.Println("Failed to connect to PostgreSQL:", err)
		return
	}
	defer db.Close()

	// Initialize Casbin enforcer
	enforcer, err = InitEnforcer(db)
	if err != nil {
		fmt.Println("Failed to initialize Casbin enforcer:", err)
		return
	}

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes for signup, login, and subscription proxy
	e.POST("/signup", SignupHandler)
	e.POST("/signin", LoginHandler)
	e.POST("/centrifugo/subscribe", SubscriptionHandler)

	// Start the server
	e.Logger.Fatal(e.Start(":9080"))
	fmt.Println("No")
}
