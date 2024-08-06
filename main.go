package main

import (
    "net/http"
    "pay2post/models"
    "github.com/gin-gonic/gin"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "log"
    "github.com/stripe/stripe-go/v72"
    "github.com/stripe/stripe-go/v72/charge"
    "golang.org/x/crypto/bcrypt"
)

var db *gorm.DB

func initDB() {
    var err error
    db, err = gorm.Open(sqlite.Open("pay2post.db"), &gorm.Config{})
    if err != nil {
        log.Fatal("failed to connect database")
    }

    db.AutoMigrate(&models.User{}, &models.Post{})
}

func main() {
    initDB()

    r := gin.Default()

    r.POST("/register", register)
    r.POST("/login", login)
    r.POST("/posts", createPost)
    r.POST("/pay", paymentHandler)
    r.GET("/posts", getPosts)
    r.PUT("/posts/:id", updatePost)
    r.DELETE("/posts/:id", deletePost)

    r.Run(":8080")
}

func register(c *gin.Context) {
    var user models.User
    if err := c.ShouldBindJSON(&user); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
        return
    }
    user.Password = string(hashedPassword)

    if err := db.Create(&user).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "User registered successfully"})
}

func login(c *gin.Context) {
    var user models.User
    if err := c.ShouldBindJSON(&user); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var dbUser models.User
    if err := db.Where("username = ?", user.Username).First(&dbUser).Error; err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
        return
    }

    if err := bcrypt.CompareHashAndPassword([]byte(dbUser.Password), []byte(user.Password)); err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
}

func createPost(c *gin.Context) {
    var post models.Post
    if err := c.ShouldBindJSON(&post); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    post.Paid = false // Initially set as unpaid

    if err := db.Create(&post).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
        return
    }

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

    // Set your secret key. Remember to switch to your live secret key in production!
    // See your keys here: https://dashboard.stripe.com/apikeys
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

    // Mark the post as paid
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
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve posts"})
        return
    }

    c.JSON(http.StatusOK, posts)
}

func updatePost(c *gin.Context) {
    var post models.Post
    if err := c.ShouldBindJSON(&post); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if err := db.Save(&post).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update post"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Post updated successfully"})
}

func deletePost(c *gin.Context) {
    var post models.Post
    if err := db.Where("id = ?", c.Param("id")).First(&post).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
        return
    }

    if err := db.Delete(&post).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete post"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}
