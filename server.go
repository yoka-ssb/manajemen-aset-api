package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"asset-management-api/app/auth"
	"asset-management-api/app/database"
	"asset-management-api/app/services"
	"asset-management-api/app/utils"
	"asset-management-api/assetpb" // Import the generated protobuf package

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

func main() {
	// Initialize database
	db := database.DBConn()

	// Create services
	services := []services.InterfaceService{
		services.NewAuthService(db),
		services.NewAssetService(db),
		services.NewUserService(db),
		services.NewAreaService(db),
		services.NewOutletService(db),
		services.NewClassificationService(db),
		services.NewMaintenancePeriodService(db),
		services.NewRoleService(db),
		services.NewPersonalResponsibleService(db),
		services.NewSubmissionService(db),
	}

	// Start the gRPC server
	go startGRPCServer(services)

	// Start the HTTP server
	go startRESTServer()

	// Start the HTTP Gateway
	startHTTPGateway()
}

func startGRPCServer(services []services.InterfaceService) {
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Load environment variables
	err = godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	jwtSecret := os.Getenv("JWT_SECRET")

	// Add JWT middleware to the gRPC server
	excludedMethods := []string{"/asset.AUTHService/Login"}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(auth.JWTAuthMiddleware(jwtSecret, excludedMethods)))

	// Dynamically register all services
	for _, svc := range services {
		svc.Register(grpcServer)
	}

	log.Println("Serving gRPC on :50053")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func startHTTPGateway() {
	ctx := context.Background()
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}

	// Regoster gRPC-Gateway handlers
	services := []struct {
		name string
		fn   func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error
	}{
		{"AUTHService", assetpb.RegisterAUTHServiceHandlerFromEndpoint},
		{"ASSETService", assetpb.RegisterASSETServiceHandlerFromEndpoint},
		{"USERService", assetpb.RegisterUSERServiceHandlerFromEndpoint},
		{"AREAService", assetpb.RegisterAREAServiceHandlerFromEndpoint},
		{"OUTLETService", assetpb.RegisterOUTLETServiceHandlerFromEndpoint},
		{"CLASSIFICATIONService", assetpb.RegisterCLASSIFICATIONServiceHandlerFromEndpoint},
		{"MAINTENANCEPERIODService", assetpb.RegisterMAINTENANCEPERIODServiceHandlerFromEndpoint},
		{"ROLEService", assetpb.RegisterROLEServiceHandlerFromEndpoint},
		{"PERSONALRESPONSIBLEService", assetpb.RegisterPERSONALRESPONSIBLEServiceHandlerFromEndpoint},
		{"SUBMISSIONService", assetpb.RegisterSUBMISSIONServiceHandlerFromEndpoint},
	}
	
	for _, svc := range services {
		err := svc.fn(ctx, mux, "localhost:50053", opts)
		if err != nil {
			log.Fatalf("Failed to start HTTP gateway for %s: %v", svc.name, err)
		}
	}

	log.Println("Serving HTTP Gateway on :8080")
	if err := http.ListenAndServe(":8080", corsHandler(mux)); err != nil {
		log.Fatalf("HTTP Gateway failed: %v", err)
	}
}

func startRESTServer() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	apiKeysEnv := os.Getenv("API_KEYS")

	// Split the API keys into a slice
	apiKeysSlice := []string{}
	for _, key := range strings.Split(apiKeysEnv, ",") {
		apiKeysSlice = append(apiKeysSlice, strings.TrimSpace(key))
	}

	// Convert the slice to a map
	apiKeys := make(map[string]bool)
	for _, key := range apiKeysSlice {
		apiKeys[key] = true
	}

	r := gin.Default()
	r.Use(auth.APIKeyMiddleware(apiKeys))

	// Upload file to Nextcloud
	r.POST("/upload", func(c *gin.Context) {
		module := c.DefaultQuery("module", "")
		filePath, err := utils.UploadFile(c.Writer, c.Request, module)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "File uploaded successfully",
			"file_path": filePath,
		})
	})

	r.GET("/get-file", func(c *gin.Context) {
		filePath := c.DefaultQuery("path", "")
		res, err := utils.GetFile(c.Writer, c.Request, filePath)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "File downloaded successfully",
			"file":    res,
		})
	})

	log.Println("Server REST started on port 8081")
	r.Run(":8081")
}

func corsHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}
