package auth

import (
	"asset-management-api/app/database"
	"asset-management-api/assetpb"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var db = database.DBConn()

func ValidateToken(tokenString string) *string {
	// Get token from database
	var tokenStore assetpb.TokenStore
	result := db.Where("token = ?", tokenString).First(&tokenStore)
	if result.Error != nil {
		return nil
	}
	
	return &tokenStore.Token
}

func JWTAuthMiddleware(jwtSecret string, excludeMethods []string) grpc.UnaryServerInterceptor {

    // Get the JWT secret from environment
    if jwtSecret == "" {
        log.Fatal("JWT_SECRET is not set")
    }

    // Return the actual middleware function
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        // Check if the current method should be excluded
        for _, excludedMethod := range excludeMethods {
            if info.FullMethod == excludedMethod {
                return handler(ctx, req)
            }
        }
        
        // Extract metadata from the incoming context
        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            return nil, fmt.Errorf("failed to extract metadata from incoming context")
        }

        // Get the token from the Authorization header
        tokenMeta := md.Get("authorization")
        tokenString := strings.Replace(tokenMeta[0], "Bearer ", "", 1)
        if len(tokenString) == 0 {
            return nil, fmt.Errorf("Unauthorized: token is missing")
        }

        // Find token from database
        findToken := ValidateToken(tokenString)

        // Parse the token
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            return []byte(jwtSecret), nil
        })
        if err != nil || !token.Valid || findToken == nil {
            return nil, status.Error(codes.Unauthenticated, "invalid token")
        }
        
        // Token is valid, proceed with the handler
        return handler(ctx, req)
    }
}

func GenerateJWTToken(nip int32) *string {
    godotenv.Load(".env")
    // Set the JWT secret key
    jwtSecret := os.Getenv("JWT_SECRET")

    // Get user by NIP
    var user assetpb.User
    err := db.Where("nip = ?", nip).First(&user).Error
    if err != nil {
        fmt.Println(err)
        return nil
    }

    // Set the token claims
    claims := jwt.MapClaims{
        "sub": nip,
        "name": user.UserFullName,
        "role_id": user.RoleId,
        "outlet_id": user.OutletId,
		"area_id" : user.AreaId,
        "exp": time.Now().Add(time.Hour * 72).Unix(),
    }

    // Generate the token
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte(jwtSecret))
    if err != nil {
        fmt.Println(err)
        return nil
    }

    fmt.Println("generated token:", tokenString)

    return &tokenString
}

// APIKeyMiddleware is a middleware that checks for the presence of a valid API key
func APIKeyMiddleware(apiKeys map[string]bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-KEY")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing API key"})
			c.Abort()
			return
		}

		if !apiKeys[apiKey] {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// func ValidateAuth(request *dto.AuthRequestDto, refreshTokenString string) (*dto.AuthResponseDto, error) {
// 	var user *models.User
// 	var err error

// 	if request != nil {
// 		s.logger.Info("Getting user data")
// 		user, err = s.userService.GetOneByUsername(request.Username)
// 		if err != nil {
// 			return nil, err
// 		}

// 		s.logger.Info("Validating user password")
// 		if !utils.CheckPasswordHash(user.Password, request.Password) {
// 			return nil, httperror.NewHttpError("invalid email or password", "", http.StatusBadRequest)
// 		}
// 	} else {

// 		s.logger.Info("Validating refresh token")
// 		authIdentity, err := s.Authorize(refreshTokenString, constants.TypeRefreshToken)
// 		if err != nil {
// 			return nil, err
// 		}

// 		err = s.tokenStoreService.ValidateToken(refreshTokenString)
// 		if err != nil {
// 			return nil, httperror.NewHttpError("invalid refresh token", "", http.StatusUnauthorized)
// 		}

// 		// Delete old token
// 		err = s.tokenStoreService.DeleteToken(refreshTokenString)
// 		if err != nil {
// 			return nil, err
// 		}

// 		s.logger.Info("Getting user data")
// 		userId, _ := strconv.Atoi(authIdentity.UserID)
// 		user, err = s.userService.GetOneUser(userId)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	s.logger.Debug("User data: ", user)

// 	s.logger.Info("Generating auth token")
// 	token, err := s.CreateToken(user, constants.TypeAuthToken)
// 	if err != nil {
// 		return nil, err
// 	}

// 	s.logger.Info("Generating refresh token")
// 	refreshToken, err := s.CreateToken(user, constants.TypeRefreshToken)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// store new refresh token
// 	var tokenStore = models.TokenStore{
// 		Token: refreshToken,
// 	}
// 	err = s.tokenStoreService.CreateToken(tokenStore)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &dto.AuthResponseDto{
// 		Token:        token,
// 		RefreshToken: refreshToken,
// 	}, nil
// }