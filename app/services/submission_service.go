package services

import (
	"asset-management-api/assetpb"
	"context"
	"errors"
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
	SubmissionQuantity    int32 `json:"submission_quantity,omitempty"`
	SubmissionAssetName   string `json:"submission_asset_name,omitempty"`
	SubmissionDescription string `json:"submission_description,omitempty"`
	Nip                   int32  `json:"nip,omitempty"`
	AssetId               int32  `json:"asset_id,omitempty"`
	Attachment            string `json:"attachment,omitempty"`
	SubmissionPrName      string `json:"submission_pr_name,omitempty"`
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
	// Validate asset
	asset, err := GetAssetById(req.AssetId)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Error(codes.NotFound, "Asset not found")
		} else {
			return nil, status.Error(codes.Internal, "Failed to get asset")
		}
	}

	// Validate asset status
	if asset.AssetStatus != "Baik" {
		return nil, status.Error(codes.NotFound, "Asset already submitted")
	}

	// Validate asset name
	if asset.AssetName != req.SubmissionAssetName {
		return nil, status.Error(codes.NotFound, "Asset name not match")
	}

	if req.SubmissionOutlet != "" {
		// Validate outlet
		if asset.OutletName != req.SubmissionOutlet {
			return nil, status.Error(codes.NotFound, "Outlet not match")
		}
	}

	if req.SubmissionArea != "" {
		// Validate area
		if asset.AreaName != req.SubmissionArea {
			return nil, status.Error(codes.NotFound, "Area not match")
		}
	}

	// Validate personal responsible 
	if asset.PersonalName != req.SubmissionPrName {
		return nil, status.Error(codes.NotFound, "Personal Responsible not match")
	}

	// Validate PIC name
	if asset.AssetPicName != req.SubmissionRoleName {
		return nil, status.Error(codes.NotFound, "PIC name not match")
	}

	var lastSubmission Submission
	last := db.Model(&assetpb.Submission{}).Last(&lastSubmission)
	if last.Error != nil {
		log.Println("Error:", last.Error)

		if errors.Is(last.Error, gorm.ErrRecordNotFound) {
			// Create first submission
			lastSubmission.SubmissionId = 0
		} else {
			return nil, status.Error(codes.Internal, "Failed to get submission: "+last.Error.Error())
		}
	}
	lastID := lastSubmission.SubmissionId
	// convert time to string
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
		Attachment:            req.Attachment,
		SubmissionPrName:      req.SubmissionPrName,
	}

	result := db.Create(&submission)
	if result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create asset: "+result.Error.Error())
	}

	// Create submission log
	submissionLog := assetpb.SubmissionLog{
		SubmissionId: submission.SubmissionId,
		Status:       "Diajukan",
		Description:  "Pengajuan dibuat oleh " + req.SubmissionName,
		PrName:       req.SubmissionPrName,
	}
	result = db.Create(&submissionLog)
	if result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to create submission log: "+result.Error.Error())
	}

	// Update asset status
	result = db.Model(&assetpb.Asset{}).Where("asset_id = ?", req.AssetId).Update("asset_status", req.SubmissionCategory)
	if result.Error != nil {
		return nil, status.Error(codes.Internal, "Failed to update asset: "+result.Error.Error())
	}

	return &assetpb.CreateSubmissionResponse{
		Message: "Successfully creating submission",
		Code:    "200",
		Success: true,
	}, nil
}
