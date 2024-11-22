package services

import (
	"asset-management-api/app/auth"
	"asset-management-api/app/utils"
	"asset-management-api/assetpb"
	"context"
	"log"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type AuthService struct {
	MasterService
	assetpb.UnimplementedAUTHServiceServer
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{
		MasterService: MasterService{DB: db}}
}

func (s *AuthService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterAUTHServiceServer(grpcServer, s)
}

func (s *AuthService) tokenStore(tokenString string) error {
	var TokenStore assetpb.TokenStore
	// Save token to database
	TokenStore.Token = tokenString
	err := db.Create(&TokenStore).Error
	if err != nil {
		return err
	}
	return nil
}

func (s *AuthService) deleteToken(tokenString string) error {

	// Delete token from database
	err := db.Where("token = ?", tokenString).Delete(&assetpb.TokenStore{}).Error
	if err != nil {
		return err
	}
	return nil
}

func (s *AuthService) GetToken(tokenString string) *string {
	// Get token from database
	var tokenStore assetpb.TokenStore
	result := db.Where("token = ?", tokenString).First(&tokenStore)
	if result.Error != nil {
		return nil
	}
	
	return &tokenStore.Token
}

func (s *AuthService) Login(ctx context.Context, req *assetpb.LoginRequest) (*assetpb.LoginResponse, error) {
	log.Default().Println("Logging in")

	// Getting user by nip
	var user assetpb.User
	err := db.Where("nip = ?", req.GetNip()).First(&user).Error
	if err != nil {
		return &assetpb.LoginResponse{
			Message: err.Error(),
			Code: "400",
			Success: false,
		}, err
	}

	// Verify password
	err = utils.VerifyPassword(user.GetUserPassword(), req.GetUserPassword())
	if err != nil {
		return &assetpb.LoginResponse{
			Message: err.Error(),
			Code: "400",
			Success: false,
		}, err
	}

	// Generate token
	token := auth.GenerateJWTToken(user.GetNip())

	// Save token to database
	err = s.tokenStore(*token)
	if err != nil {
		return &assetpb.LoginResponse{
			Message: err.Error(),
			Code: "400",
			Success: false,
		}, err
	}

	return &assetpb.LoginResponse{
		Message: "Successfully logged in",
		Code: "200",
		Token: *token,
		Success: true,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, req *assetpb.LogoutRequest) (*assetpb.LogoutResponse, error) {
	log.Default().Println("Logging out")

	// Delete token
	err := s.deleteToken(req.Token)
	if err != nil {
		return &assetpb.LogoutResponse{
			Message: "Failed to logout",
			Code: "400",
			Success: false,
		}, nil
	}
	
	return &assetpb.LogoutResponse{
		Message: "Successfully logged out",
		Code: "200",
		Success: true,
	}, nil
}