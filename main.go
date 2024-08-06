package main

import (
	"log"
	"net/http"
	"pay2post/models"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/charge"
	"go.uber.org/zap" // Import Zap
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	db     *gorm.DB
	logger *zap.Logger // Declare a logger variable
)

func initDB() {
	var err error
	db, err = gorm.Open(sqlite.Open("pay2post.db"), &gorm.Config{})
	if err != nil {
		logger.Fatal("failed to connect database", zap.Error(err)) // Use Zap logger
	}

	db.AutoMigrate(&models.User{}, &models.Post{})
}

func main() {
	// Initialize Zap logger
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		log.Fatal("cannot create logger", err)
	}
	defer logger.Sync() // Flushes buffer, if any

	initDB()

	r := gin.Default()

	// Load HTML templates
	r.LoadHTMLGlob("templates/*")

	// Serve the index page
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.POST("/register", register)
	r.POST("/login", login)
	r.POST("/posts", createPost) // Payment handling removed
	r.POST("/pay", paymentHandler) // Payment endpoint
	r.GET("/posts", getPosts)
	r.PUT("/posts/:id", updatePost)
	r.DELETE("/posts/:id", deletePost)

	// log the http://localhost:8080
	logmessage := "please visit http://localhost:8080"
	logger.Info(logmessage)

	r.Run(":80")
}

func register(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		logger.Warn("register failed", zap.String("error", err.Error())) // Log warning
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("failed to hash password", zap.Error(err)) // Log error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	user.Password = string(hashedPassword)

	if err := db.Create(&user).Error; err != nil {
		logger.Error("failed to create user", zap.Error(err)) // Log error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	logger.Info("user registered successfully", zap.String("username", user.Username)) // Log info
	c.JSON(http.StatusOK, gin.H{"message": "User registered successfully"})
}

func login(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		logger.Warn("login failed", zap.String("error", err.Error())) // Log warning
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var dbUser models.User
	if err := db.Where("username = ?", user.Username).First(&dbUser).Error; err != nil {
		logger.Warn("invalid username or password", zap.String("username", user.Username)) // Log warning
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(dbUser.Password), []byte(user.Password)); err != nil {
		logger.Warn("invalid username or password", zap.String("username", user.Username)) // Log warning
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	logger.Info("login successful", zap.String("username", user.Username)) // Log info
	c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
}

func createPost(c *gin.Context) {
	var post models.Post
	if err := c.ShouldBindJSON(&post); err != nil {
		logger.Warn("create post failed", zap.String("error", err.Error())) // Log warning
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post.Paid = false // Initially set as unpaid

	if err := db.Create(&post).Error; err != nil {
		logger.Error("failed to create post", zap.Error(err)) // Log error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	logger.Info("post created successfully", zap.Uint("post_id", post.ID)) // Log info
	c.JSON(http.StatusOK, post)
}

func paymentHandler(c *gin.Context) {
	var input struct {
		Token  string `json:"token"`
		PostID uint   `json:"post_id"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stripe.Key = "sk_test_YOUR_SECRET_KEY"

	params := &stripe.ChargeParams{
		Amount:      stripe.Int64(500), // Amount in cents
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String("Pay2Post charge"),
	}
	params.SetSource(input.Token)

	_, err := charge.New(params)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var post models.Post
	if err := db.First(&post, input.PostID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	post.Paid = true
	db.Save(&post)

	c.JSON(http.StatusOK, gin.H{"message": "Payment successful and post updated"})
}

func getPosts(c *gin.Context) {
	var posts []models.Post
	if err := db.Find(&posts).Error; err != nil {
		logger.Error("failed to retrieve posts", zap.Error(err)) // Log error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve posts"})
		return
	}

	logger.Info("posts retrieved successfully", zap.Int("num_posts", len(posts))) // Log info
	c.JSON(http.StatusOK, posts)
}

func updatePost(c *gin.Context) {
	var post models.Post
	if err := c.ShouldBindJSON(&post); err != nil {
		logger.Warn("update post failed", zap.String("error", err.Error())) // Log warning
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.Save(&post).Error; err != nil {
		logger.Error("failed to update post", zap.Error(err)) // Log error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update post"})
		return
	}

	logger.Info("post updated successfully", zap.Uint("post_id", post.ID)) // Log info
	c.JSON(http.StatusOK, gin.H{"message": "Post updated successfully"})
}

func deletePost(c *gin.Context) {
	var post models.Post
	if err := db.Where("id = ?", c.Param("id")).First(&post).Error; err != nil {
		logger.Warn("post not found", zap.String("post_id", c.Param("id"))) // Log warning
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	if err := db.Delete(&post).Error; err != nil {
		logger.Error("failed to delete post", zap.Error(err)) // Log error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete post"})
		return
	}

	logger.Info("post deleted successfully", zap.Uint("post_id", post.ID)) // Log info
	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}
