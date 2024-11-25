package services

import (
	"asset-management-api/app/auth"
	"asset-management-api/app/utils"
	"asset-management-api/assetpb"
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type UserService struct {
	MasterService
	assetpb.UnimplementedUSERServiceServer
}

type User struct {
	Nip          int32  `json:"nip"`
	UserFullName string `json:"user_full_name"`
	UserEmail    string `json:"user_email"`
	UserPassword string `json:"user_password"`
	RoleID       int32  `json:"role_id"`
	AreaID       *int32 `json:"area_id"`
	OutletID     *int32 `json:"outlet_id"`
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{
		MasterService: MasterService{DB: db}}
}

func (s *UserService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterUSERServiceServer(grpcServer, s)
}

func (s *UserService) CreateUser(ctx context.Context, req *assetpb.CreateUserRequest) (*assetpb.CreateUserResponse, error) {

	if req == nil {
		return &assetpb.CreateUserResponse{
			Message: "Missing request body",
			Code:    "400",
			Success: false}, nil
	}
	log.Default().Println("Creating new user")

	hashedPassword, err := utils.HashPassword(req.GetUserPassword())
	if err != nil {
		return &assetpb.CreateUserResponse{
			Message: err.Error(),
			Code:    "400",
			Success: false}, nil
	}

	var areaId *int32
	if req.AreaId != 0 {
		areaId = &req.AreaId
	}

	var outletId *int32
	if req.OutletId != 0 {
		outletId = &req.OutletId
	}

	user := User{
		Nip:          req.GetNip(),
		UserFullName: req.GetUserFullName(),
		UserEmail:    req.GetUserEmail(),
		UserPassword: hashedPassword,
		RoleID:       req.GetRoleId(),
		AreaID:       areaId,
		OutletID:     outletId,
	}

	err = db.Create(&user).Error
	if err != nil {
		return &assetpb.CreateUserResponse{
			Message: err.Error(),
			Code:    "400",
			Success: false}, nil
	}

	fmt.Printf("New user ID: %d\n", user.Nip)

	return &assetpb.CreateUserResponse{
		Message: "Suceccfully created user",
		Code:    "200",
		Success: true}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *assetpb.GetUserRequest) (*assetpb.GetUserResponse, error) {
	log.Default().Println("Getting user with nip: ", req.GetNip())
	var user assetpb.User
	err := db.Select(
		"nip",
		"user_full_name",
		"user_email",
		"role_id",
		"area_id",
		"outlet_id").Where("nip = ?", req.GetNip()).First(&user).Error
	if err != nil {
		return &assetpb.GetUserResponse{
			Message: "User not found",
			Code:    "400",
			Success: false}, nil
	}

	return &assetpb.GetUserResponse{
		Message: "Suceccfully created user",
		Code:    "200",
		Data:    &user,
		Success: true,
	}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, req *assetpb.UpdateUserRequest) (*assetpb.UpdateUserResponse, error) {

	updates := map[string]interface{}{
		"UserFullName": req.GetUserFullName(),
		"UserEmail":    req.GetUserEmail(),
		"RoleId":       req.GetRoleId(),
		"AreaId":       req.GetAreaId(),
		"OutletId":     req.GetOutletId(),
	}

	err := db.Model(&assetpb.User{}).Where("nip = ?", req.Nip).Updates(updates).Error
	if err != nil {
		return &assetpb.UpdateUserResponse{
			Message: err.Error(),
			Code:    "400",
			Success: false}, nil
	}

	return &assetpb.UpdateUserResponse{
		Message: "Suceccfully created user",
		Code:    "200",
		Success: true}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, req *assetpb.DeleteUserRequest) (*assetpb.DeleteUserResponse, error) {
	log.Default().Println("Deleting user")
	err := db.Delete(&assetpb.User{}, req.GetNip()).Error
	if err != nil {
		return &assetpb.DeleteUserResponse{Success: false}, nil
	}
	return &assetpb.DeleteUserResponse{Success: true}, nil
}

func (s *UserService) ListUsers(ctx context.Context, req *assetpb.ListUsersRequest) (*assetpb.ListUsersResponse, error) {
	log.Default().Println("Listing users")
	// Get the page number and page size from the request
	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()

	// Calculate the offset and limit for the query
	offset := (pageNumber - 1) * pageSize
	limit := pageSize

	// Get the users from the database
	users, err := getUsers(offset, limit, q)
	if err != nil {
		log.Default().Println("Error fetching users:", err)
		return nil, err
	}

	// Get the total count of users
	totalCount, err := getTotalCount()
	if err != nil {
		log.Default().Println("Error fetching total count:", err)
		return nil, err
	}

	// Create a response
	resp := &assetpb.ListUsersResponse{
		Data:       users,
		TotalCount: totalCount,
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	// Calculate the next page token
	if totalCount > offset+limit {
		resp.NextPageToken = fmt.Sprintf("page_token_%d", pageNumber+1)
	}

	return resp, nil
}

func (s *UserService) ResetPassword(ctx context.Context, req *assetpb.ResetPasswordRequest) (*assetpb.ResetPasswordResponse, error) {
	log.Default().Println("Resetting password")
	// Validate user
	_, err := s.GetUser(ctx, &assetpb.GetUserRequest{Nip: req.GetNip()})
	if err != nil {
		return &assetpb.ResetPasswordResponse{
			Message: "User not foundr",
			Code:    "404",
			Success: false}, nil
	}

	// Validate token
	token := auth.ValidateToken(req.GetResetToken())
	if token == nil {
		return &assetpb.ResetPasswordResponse{
			Message: "Invalid reset token",
			Code:    "400",
			Success: false}, nil
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.GetUserPassword())
	if err != nil {
		return &assetpb.ResetPasswordResponse{
			Message: "Failed to hash password",
			Code:    "400",
			Success: false}, nil
	}

	// Reset password: update user password
	err = db.Model(&assetpb.User{}).Where("nip = ?", req.GetNip()).Update("user_password", hashedPassword).Error
	if err != nil {
		return &assetpb.ResetPasswordResponse{
			Message: "Failed to reset password",
			Code:    "400",
			Success: false}, nil
	}

	return &assetpb.ResetPasswordResponse{
		Message: "Successfully reset password",
		Code:    "200",
		Success: true}, nil
}

func getUsers(offset, limit int32, q string) ([]*assetpb.User, error) {
	// Query the database to get the users
	var users []*assetpb.User
	var query *gorm.DB

	if q != "" {
		query = db.Raw("SELECT * FROM users WHERE user_full_name LIKE ? LIMIT ? OFFSET ?", "%"+q+"%", limit, offset)
	} else {
		query = db.Raw("SELECT * FROM users LIMIT ? OFFSET ?", limit, offset)
	}

	err := query.Find(&users).Error
	if err != nil {
		log.Default().Println("Error fetching users:", err)
		return nil, err
	}

	return users, nil
}

func getTotalCount() (int32, error) {
	// Query the database to get the total count of users
	var count int32
	err := db.Raw("SELECT COUNT(*) FROM users").Scan(&count).Error
	if err != nil {
		log.Default().Println("Error fetching total count:", err)
		return 0, err
	}

	return count, nil
}
