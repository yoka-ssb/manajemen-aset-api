package services

import (
	"asset-management-api/assetpb"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
}

func NewSubmissionService(db *gorm.DB) *SubmissionService {
	return &SubmissionService{
		MasterService: MasterService{DB: db}}
}

func (s *SubmissionService) Register(server interface{}) {
	grpcServer := server.(grpc.ServiceRegistrar)
	assetpb.RegisterSUBMISSIONServiceServer(grpcServer, s)
}

func (s *SubmissionService) CreateSubmission(ctx context.Context, req *assetpb.CreateSubmissionRequest) (*assetpb.CreateSubmissionResponse, error) {
	log.Println("Creating submission")

	// Validate asset
	asset, err := GetAssetById(req.AssetId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "Asset not found")
		}
		return nil, status.Error(codes.Internal, "Failed to get asset")
	}

	// Validate asset status and other fields
	if asset.AssetStatus != "Baik" || asset.AssetName != req.SubmissionAssetName ||
		(req.SubmissionOutlet != "" && asset.OutletName != req.SubmissionOutlet) ||
		(req.SubmissionArea != "" && asset.AreaName != req.SubmissionArea) ||
		asset.AssetPicName != req.SubmissionRoleName {
		return nil, status.Error(codes.NotFound, "Asset or related details do not match")
	}

	// Get the last Submission ID
	var lastSubmission Submission
	last := db.Model(&assetpb.Submission{}).Last(&lastSubmission)
	if last.Error != nil {
		log.Println("Error:", last.Error)
		if errors.Is(last.Error, gorm.ErrRecordNotFound) {
			lastSubmission.SubmissionId = 0
		} else {
			return nil, status.Error(codes.Internal, "Failed to get submission: "+last.Error.Error())
		}
	}
	lastID := lastSubmission.SubmissionId
	submissionDate := time.Now().Format("2006-01-02")

	submission := Submission{
		SubmissionId:          lastID + 1,
		SubmissionName:        req.SubmissionName,
		SubmissionOutlet:      req.SubmissionOutlet,
		SubmissionArea:        req.SubmissionArea,
		SubmissionDate:        submissionDate,
		SubmissionCategory:    req.SubmissionCategory,
		SubmissionStatus:      req.SubmissionStatus,
		SubmissionPurpose:     req.SubmissionPurpose,
		SubmissionAssetName:   req.SubmissionAssetName,
		SubmissionDescription: req.SubmissionDescription,
		Nip:                   req.Nip,
		AssetId:               req.AssetId,
		SubmissionPrName:      req.SubmissionPrName,
		SubmissionRoleName:    req.SubmissionRoleName,
		Attachment:            req.Attachment,
	}

	result := db.Create(&submission)
	if result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create submission: "+result.Error.Error())
	}

	// Log submission
	submissionLog := assetpb.SubmissionLog{
		SubmissionId: submission.SubmissionId,
		Status:       "Diajukan",
		Description:  "Pengajuan dibuat oleh " + req.SubmissionName,
		PrName:       req.SubmissionPrName,
	}
	if result := db.Create(&submissionLog); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create submission log: "+result.Error.Error())
	}

	// Update asset status
	if result := db.Model(&assetpb.Asset{}).Where("asset_id = ?", req.AssetId).Update("asset_status", req.SubmissionCategory); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to update asset status: "+result.Error.Error())
	}

	// Record asset update
	if result := db.Create(&assetpb.AssetUpdate{
		AssetId:     req.AssetId,
		AssetStatus: req.SubmissionCategory,
	}); result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create asset update: "+result.Error.Error())
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

func (s *SubmissionService) ListSubmissions(ctx context.Context, req *assetpb.ListSubmissionsRequest) (*assetpb.ListSubmissionsResponse, error) {
	log.Println("Listing submissions")

	pageNumber := req.GetPageNumber()
	pageSize := req.GetPageSize()
	q := req.GetQ()

	offset := (pageNumber - 1) * pageSize
	limit := pageSize

	submissions, err := GetSubmissions(offset, limit, q)
	if err != nil {
		log.Println("Error fetching submissions:", err)
		return nil, err
	}

	totalCount, err := GetTotalCount("submissions")
	if err != nil {
		log.Println("Error fetching total count:", err)
		return nil, err
	}

	resp := &assetpb.ListSubmissionsResponse{
		Data:       submissions,
		TotalCount: totalCount,
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	if totalCount > offset+limit {
		resp.NextPageToken = fmt.Sprintf("page_token_%d", pageNumber+1)
	}

	return resp, nil
}

func GetSubmissions(offset, limit int32, q string) ([]*assetpb.Submission, error) {
	var submissions []*assetpb.Submission
	query := db.Select("submissions.*, users.nip AS nip, assets.asset_id AS asset_id").
		Limit(int(limit)).
		Offset(int(offset))

	if q != "" {
		query = query.Where("submissions.submission_name LIKE ?", "%"+q+"%")
	}

	err := query.Joins("LEFT JOIN users ON users.nip = submissions.nip").
		Joins("LEFT JOIN assets ON assets.asset_id = submissions.asset_id").
		Find(&submissions).Error

	if err != nil {
		log.Println("Error fetching submissions:", err)
		return nil, err
	}

	return submissions, nil
}

func GetSubmissionById(id int32) (*assetpb.Submission, error) {
    var submission assetpb.Submission
    log.Println("Fetching submission with ID:", id)
    
    query := db.Select("submissions.*, users.nip AS nip, assets.asset_id AS asset_id").
        Joins("LEFT JOIN users ON users.nip = users.nip").
        Joins("LEFT JOIN assets ON assets.asset_id = assets.asset_id").
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

    // Panggil fungsi helper untuk mendapatkan submission dari DB.
    submission, err := GetSubmissionById(req.Id)
    if err != nil {
        log.Println("Error fetching submission:", err)
        return nil, err
    }

    return &assetpb.GetSubmissionByIdResponse{
        Submission: submission,
    }, nil
}
