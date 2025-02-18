package services

import (
	"asset-management-api/assetpb"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gorm.io/gorm"
)

type SubmissionService struct {
	MasterService
	assetpb.UnimplementedSUBMISSIONServiceServer
}

type Submission struct {
	SubmissionId          int32  `json:"submission_id,omitempty"`
	SubmissionName        string `json:"submission_name,omitempty"`
	SubmissionOutlet      string `json:"submission_outlet,omitempty"`
	SubmissionArea        string `json:"submission_area,omitempty"`
	SubmissionDate        string `json:"submission_date,omitempty"`
	SubmissionCategory    string `json:"submission_category,omitempty"`
	SubmissionStatus      string `json:"submission_status,omitempty"`
	SubmissionPurpose     string `json:"submission_purpose,omitempty"`
	SubmissionQuantity    int32  `json:"submission_quantity,omitempty"`
	SubmissionAssetName   string `json:"submission_asset_name,omitempty"`
	SubmissionDescription string `json:"submission_description,omitempty"`
	Nip                   int32  `json:"nip,omitempty"`
	AssetId               int32  `json:"asset_id,omitempty"`
	Attachment            string `json:"attachment,omitempty"`
	SubmissionPrName      string `json:"submission_pr_name,omitempty"`
	SubmissionRoleName    string `json:"submission_role_name,omitempty"`
	OutletId              int32  `json:"outlet_id,omitempty"`
	AreaId                int32  `json:"area_id,omitempty"`
	SubmissionPrice       int32  `json:"submission_price,omitempty"`
	SubmissionParentId    int32  `json:"submission_parent_id,omitempty"`
}

type SubmissionParents struct {
	SubmissionParentId uint   `json:"submission_parent_id"`
	Nip                string `json:"nip"`
	SubmissionCategory string `json:"submission_category"`
}

func NewSubmissionService(db *gorm.DB) *SubmissionService {
	return &SubmissionService{
		MasterService: MasterService{DB: db}}
}

func (s *SubmissionService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterSUBMISSIONServiceServer(grpcServer, s)
}

func sendEmail(toEmail, subject, body string) error {
	godotenv.Load(".env")

	// Setup SMTP server
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	senderEmail := os.Getenv("SENDER_EMAIL")
	senderPassword := os.Getenv("SENDER_PASSWORD")

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		senderEmail, toEmail, subject, body,
	)

	auth := smtp.PlainAuth("", senderEmail, senderPassword, smtpHost)

	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, senderEmail, []string{toEmail}, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}
	return nil
}

func (s *SubmissionService) CreateSubmission(ctx context.Context, req *assetpb.CreateSubmissionRequest) (*assetpb.CreateSubmissionResponse, error) {
	log.Println("Creating submission")

	// Cek apakah asset ada
	asset, err := GetAssetById(req.AssetId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "Asset not found")
		}
		return nil, status.Error(codes.Internal, "Failed to get asset")
	}

	// Validasi data asset dengan data pengajuan
	if asset.AssetStatus != "Baik" || asset.AssetName != req.SubmissionAssetName ||
		(req.SubmissionOutlet != "" && asset.OutletName != req.SubmissionOutlet) ||
		(req.SubmissionArea != "" && asset.AreaName != req.SubmissionArea) ||
		asset.AssetPicName != req.SubmissionRoleName {
		return nil, status.Error(codes.NotFound, "Asset or related details do not match")
	}

	// Dapatkan ID terakhir dari tabel submission
	var lastSubmission Submission
	last := db.Model(&Submission{}).Last(&lastSubmission)
	if last.Error != nil {
		if errors.Is(last.Error, gorm.ErrRecordNotFound) {
			lastSubmission.SubmissionId = 0
		} else {
			return nil, status.Error(codes.Internal, "Failed to get last submission: "+last.Error.Error())
		}
	}
	lastID := lastSubmission.SubmissionId
	submissionDate := time.Now().Format("2006-01-02")

	// Simpan data submission ke dalam tabel submissions dengan submission_parent_id
	submission := Submission{
		SubmissionId:          lastID + 1,
		SubmissionName:        req.SubmissionName,
		SubmissionOutlet:      req.SubmissionOutlet,
		OutletId:              req.OutletId,
		AreaId:                req.AreaId,
		SubmissionArea:        req.SubmissionArea,
		SubmissionDate:        submissionDate,
		SubmissionCategory:    req.SubmissionCategory,
		SubmissionStatus:      req.SubmissionStatus,
		SubmissionPurpose:     req.SubmissionPurpose,
		SubmissionAssetName:   req.SubmissionAssetName,
		SubmissionQuantity:    req.SubmissionQuantity,
		SubmissionDescription: req.SubmissionDescription,
		Nip:                   req.Nip,
		AssetId:               req.AssetId,
		SubmissionPrName:      req.SubmissionPrName,
		SubmissionRoleName:    req.SubmissionRoleName,
		Attachment:            req.Attachment,
		SubmissionPrice:       req.SubmissionPrice,
	}

	// Simpan submission ke database
	if result := db.Create(&submission); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create submission: "+result.Error.Error())
	}

	// Simpan log submission
	submissionLog := assetpb.SubmissionLog{
		SubmissionId: submission.SubmissionId,
		Status:       "Diajukan",
		Description:  "Pengajuan dibuat oleh " + req.SubmissionName,
		PrName:       req.SubmissionPrName,
	}
	if result := db.Create(&submissionLog); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create submission log: "+result.Error.Error())
	}

	// Update status asset
	if result := db.Model(&assetpb.Asset{}).Where("asset_id = ?", req.AssetId).Update("asset_status", req.SubmissionCategory); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to update asset status: "+result.Error.Error())
	}

	// Simpan perubahan status aset
	if result := db.Create(&assetpb.AssetUpdate{
		AssetId:     req.AssetId,
		AssetStatus: req.SubmissionCategory,
	}); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create asset update: "+result.Error.Error())
	}

	// Cari user dengan role tertentu
	var users []User
	err = db.Where("role_id IN (?) AND role_id NOT IN (?)", []int{1, 3, 4, 2, 7}, []int{5, 6}).Find(&users).Error
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to find users: "+err.Error())
	}

	// Persiapan email
	subject := "Pemberitahuan Pengajuan Maintenance Asset"
	body := fmt.Sprintf("Halo,\n\nPengajuan maintenance asset telah berhasil diajukan.\n\nDetail Pengajuan:\nAsset: %s\nKategori: %s\nStatus: %s\nTanggal Pengajuan: %s\n\nTerima kasih.",
		req.SubmissionAssetName, req.SubmissionCategory, req.SubmissionStatus, submissionDate)

	// Kirim email ke user yang memenuhi syarat
	for _, user := range users {
		if user.UserEmail != "" {
			if err := sendEmail(user.UserEmail, subject, body); err != nil {
				log.Println("Failed to send email to", user.UserEmail, err)
			} else {
				log.Println("Email sent to", user.UserEmail)
			}
		}
	}

	return &assetpb.CreateSubmissionResponse{
		Message: "Successfully created submission",
		Code:    "200",
		Success: true,
	}, nil
}

func (s *SubmissionService) UpdateSubmissionStatus(ctx context.Context, req *assetpb.UpdateSubmissionStatusRequest) (*assetpb.UpdateSubmissionStatusResponse, error) {
	log.Println("Updating submission status for ID:", req.Id)
	result := db.Model(&assetpb.Submission{}).Where("submission_id = ?", req.Id).Update("submission_status", req.Status)
	if result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to update submission status: "+result.Error.Error())
	}

	submission := Submission{}
	if result := db.First(&submission, req.Id); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to get submission: "+result.Error.Error())
	}

	submissionLog := assetpb.SubmissionLog{
		SubmissionId: req.Id,
		Status:       req.Status,
		Description:  "Status updated by " + submission.SubmissionName,
		PrName:       submission.SubmissionPrName,
	}
	if result := db.Create(&submissionLog); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create submission log: "+result.Error.Error())
	}

	// Update asset status
	if result := db.Model(&assetpb.Asset{}).Where("asset_id = ?", submission.AssetId).Update("asset_status", req.Status); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to update asset status: "+result.Error.Error())
	}

	// Record asset update
	if result := db.Create(&assetpb.AssetUpdate{
		AssetId:     submission.AssetId,
		AssetStatus: req.Status,
	}); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create asset update: "+result.Error.Error())
	}

	return &assetpb.UpdateSubmissionStatusResponse{
		Message: "Successfully updated submission status",
		Code:    "200",
		Success: true,
	}, nil
}

func (s *SubmissionService) ListSubmissionsHandler(c *gin.Context) {
	pageNumberParam := c.DefaultQuery("page_number", "1")
	pageSizeParam := c.DefaultQuery("page_size", "10")
	q := c.DefaultQuery("q", "")
	nipParam := c.DefaultQuery("nip", "")
	areaIDParam := c.DefaultQuery("area_id", "")
	outletIDParam := c.DefaultQuery("outlet_id", "")

	pageNumber, err := strconv.Atoi(pageNumberParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page_number"})
		return
	}

	pageSize, err := strconv.Atoi(pageSizeParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page_size"})
		return
	}

	roleID, err := s.GetRoleIDByNIP(nipParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid NIP or user not found"})
		return
	}

	var outletID *wrapperspb.Int32Value
	if outletIDParam != "" {
		outletIDInt, err := strconv.Atoi(outletIDParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid outlet_id"})
			return
		}
		outletID = wrapperspb.Int32(int32(outletIDInt))
	}

	var areaID *wrapperspb.Int32Value
	if areaIDParam != "" {
		areaIDInt, err := strconv.Atoi(areaIDParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid area_id"})
			return
		}
		areaID = wrapperspb.Int32(int32(areaIDInt))
	}

	req := &assetpb.ListSubmissionsRequest{
		PageNumber: int32(pageNumber),
		PageSize:   int32(pageSize),
		Q:          q,
		RoleId:     int32(roleID),
	}

	if areaID != nil {
		req.AreaId = areaID.GetValue()
	}
	if outletID != nil {
		req.OutletId = outletID.GetValue()
	}

	resp, err := s.ListSubmissions(context.Background(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *SubmissionService) GetRoleIDByNIP(nip string) (int32, error) {
	var user User
	err := db.Where("nip = ?", nip).First(&user).Error
	if err != nil {
		return 0, err
	}
	return user.RoleID, nil
}

func (s *SubmissionService) ListSubmissions(ctx context.Context, req *assetpb.ListSubmissionsRequest) (*assetpb.ListSubmissionsResponse, error) {
	log.Println("Listing submissions")

	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()
	roleID := req.GetRoleId()
	areaID := req.GetAreaId()
	outletID := req.GetOutletId()

	offset := (pageNumber - 1) * pageSize
	limit := pageSize

	submissions, err := GetSubmissions(offset, limit, q, roleID, areaID, outletID)
	if err != nil {
		log.Println("Error fetching submissions:", err)
		return nil, err
	}

	totalCount, err := GetTotalCount("submissions")

	totalPengabaianKondisiAset, err := GetTotalCountByCategory("Pengabaian Kondisi Aset")
	if err != nil {
		log.Println("Error fetching total count for Pengabaian Kondisi Aset:", err)
		return nil, err
	}

	totalLaporanBarangHilang, err := GetTotalCountByCategory("Laporan Barang Hilang")
	if err != nil {
		log.Println("Error fetching total count for Laporan Barang Hilang:", err)
		return nil, err
	}

	totalPengajuanService, err := GetTotalCountByCategory("Pengajuan Service")
	if err != nil {
		log.Println("Error fetching total count for Pengajuan Service:", err)
		return nil, err
	}

	totalPengajuanGanti, err := GetTotalCountByCategory("Pengajuan Ganti")
	if err != nil {
		log.Println("Error fetching total count for Pengajuan Ganti:", err)
		return nil, err
	}

	resp := &assetpb.ListSubmissionsResponse{
		Data:                       submissions,
		TotalCount:                 totalCount,
		PageNumber:                 pageNumber,
		PageSize:                   pageSize,
		TotalPengabaianKondisiAset: totalPengabaianKondisiAset,
		TotalLaporanBarangHilang:   totalLaporanBarangHilang,
		TotalPengajuanService:      totalPengajuanService,
		TotalPengajuanGanti:        totalPengajuanGanti,
	}

	if totalCount > offset+limit {
		resp.NextPageToken = fmt.Sprintf("%d", pageNumber+1)
	} else {
		resp.NextPageToken = ""
	}

	return resp, nil
}

func GetTotalCountByCategory(category string) (int32, error) {
	var count int64
	err := db.Model(&Submission{}).Where("submission_category = ?", category).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func GetSubmissions(offset, limit int32, q string, roleID int32, areaID int32, outletID int32) ([]*assetpb.Submission, error) {
	var submissions []*assetpb.Submission
	query := db.Select("submissions.*, assets.asset_id AS asset_id").
		Joins("LEFT JOIN assets ON assets.asset_id = submissions.asset_id").
		Limit(int(limit)).
		Offset(int(offset)).
		Order("submissions.submission_date ASC")

	if q != "" {
		query = query.Where("submissions.submission_name LIKE ?", "%"+q+"%")
	}
	if roleID == 5 && areaID != 0 {
		query = query.Where("submissions.area_id = ?", areaID)
	}
	if roleID == 6 && outletID != 0 {
		query = query.Where("submissions.outlet_id = ?", outletID)
	}
	log.Printf("roleID: %d, areaID: %d, outletID: %d", roleID, areaID, outletID)

	result := query.Find(&submissions)
	if result.Error != nil {
		return nil, result.Error
	}

	return submissions, nil
}

func GetSubmissionById(id int32) (*assetpb.Submission, error) {
	var submission assetpb.Submission
	log.Println("Fetching submission with ID:", id)

	query := db.Select("submissions.*, assets.asset_id AS asset_id, assets.outlet_id, assets.area_id").
		Joins("LEFT JOIN assets ON assets.asset_id = submissions.asset_id").
		Where("submissions.submission_id = ?", id)

	result := query.First(&submission)
	if result.Error != nil {
		log.Println("Error fetching submission:", result.Error)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "Submission not found")
		} else {
			return nil, status.Error(codes.Internal, "Failed to get submission")
		}
	}
	return &submission, nil
}

func (s *SubmissionService) GetSubmissionById(ctx context.Context, req *assetpb.GetSubmissionByIdRequest) (*assetpb.GetSubmissionByIdResponse, error) {
	log.Println("Fetching submission with ID:", req.Id)

	submission, err := GetSubmissionById(req.Id)
	if err != nil {
		log.Println("Error fetching submission:", err)
		return nil, err
	}

	return &assetpb.GetSubmissionByIdResponse{
		Submission: submission,
	}, nil
}
