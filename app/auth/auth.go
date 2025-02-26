package auth

import (
    "asset-management-api/app/database"
    "asset-management-api/assetpb"
    "context"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/dgrijalva/jwt-go"
    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

var db *pgxpool.Pool

func init() {
    db = database.DBConn()
}

func ValidateToken(tokenString string) *string {
    var token string
    query := "SELECT token FROM token_stores WHERE token = $1" // PostgreSQL uses $1
    err := db.QueryRow(context.Background(), query, tokenString).Scan(&token)
    if err != nil {
        log.Error().Err(err).Msg("Failed to validate token")
        return nil
    }
    return &token
}

func JWTAuthMiddleware(jwtSecret string, excludeMethods []string) grpc.UnaryServerInterceptor {
    if jwtSecret == "" {
        log.Fatal().Msg("JWT_SECRET is not set")
    }

    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        for _, excludedMethod := range excludeMethods {
            if info.FullMethod == excludedMethod {
                return handler(ctx, req)
            }
        }

        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            log.Error().Msg("Failed to extract metadata from incoming context")
            return nil, status.Error(codes.Unauthenticated, "Failed to extract metadata")
        }

        tokenMeta := md.Get("authorization")
        if len(tokenMeta) == 0 {
            log.Warn().Msg("Unauthorized: token is missing")
            return nil, status.Error(codes.Unauthenticated, "Unauthorized: token is missing")
        }

        tokenString := strings.Replace(tokenMeta[0], "Bearer ", "", 1)
        if len(tokenString) == 0 {
            log.Warn().Msg("Unauthorized: token is missing")
            return nil, status.Error(codes.Unauthenticated, "Unauthorized: token is missing")
        }

        findToken := ValidateToken(tokenString)
        if findToken == nil {
            log.Error().Msg("Token not found in database")
            return nil, status.Error(codes.Unauthenticated, "Invalid token")
        }

        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            return []byte(jwtSecret), nil
        })
        if err != nil || !token.Valid {
            log.Error().Msg("Invalid token")
            return nil, status.Error(codes.Unauthenticated, "Invalid token")
        }

        return handler(ctx, req)
    }
}

func GenerateJWTToken(nip int32) *string {
    jwtSecret := os.Getenv("JWT_SECRET")
    if jwtSecret == "" {
        log.Error().Msg("JWT_SECRET is not set")
        return nil
    }

    var user assetpb.User
    query := "SELECT user_full_name, role_id, outlet_id, area_id FROM users WHERE nip = $1"
    err := db.QueryRow(context.Background(), query, nip).Scan(&user.UserFullName, &user.RoleId, &user.OutletId, &user.AreaId)
    if err != nil {
        log.Error().Err(err).Msg("Failed to fetch user data")
        return nil
    }

    claims := jwt.MapClaims{
        "sub":       nip,
        "name":      user.UserFullName,
        "role_id":   user.RoleId,
        "outlet_id": user.OutletId,
        "area_id":   user.AreaId,
        "exp":       time.Now().Add(time.Hour * 72).Unix(),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte(jwtSecret))
    if err != nil {
        log.Error().Err(err).Msg("Failed to generate JWT token")
        return nil
    }

    log.Info().Str("token", tokenString).Msg("Generated JWT token")
    return &tokenString
}

func APIKeyMiddleware(apiKeys map[string]bool) gin.HandlerFunc {
    return func(c *gin.Context) {
        apiKey := c.GetHeader("X-API-KEY")
        if apiKey == "" {
            log.Warn().Msg("Missing API key")
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing API key"})
            c.Abort()
            return
        }

        if !apiKeys[apiKey] {
            log.Warn().Str("api_key", apiKey).Msg("Invalid API key")
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
            c.Abort()
            return
        }

        log.Info().Str("api_key", apiKey).Msg("API key validated successfully")
        c.Next()
    }
}