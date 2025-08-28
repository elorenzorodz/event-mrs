package main

import (
	"log"
	"net/http"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load(".env.dev")

	port := common.GetEnvVariable("PORT")
	apiVersion := common.GetEnvVariable("API_VERSION")
	ginMode := common.GetEnvVariable("GIN_MODE")

	gin.SetMode(ginMode)

	router := gin.Default()
	v1 := router.Group("/" + apiVersion)
	{
		v1.GET("/ping", func(ginContext *gin.Context) {
			ginContext.JSON(http.StatusOK, gin.H { "message": "pong" })
		})
	}

	log.Printf("Server starting on port %s in %s mode", port, ginMode)

	routerRunError := router.Run(":" + port)

	if routerRunError != nil {
		log.Fatal(routerRunError)
	}
}
