package services

import (
    "asset-management-api/assetpb"
    "context"
    "github.com/rs/zerolog/log"
    "google.golang.org/grpc"
    "gorm.io/gorm"
)

type OutletService struct {
    MasterService
    assetpb.UnimplementedOUTLETServiceServer
}

func NewOutletService(db *gorm.DB) *OutletService {
    return &OutletService{
        MasterService: MasterService{DB: db}}
}

func (s *OutletService) Register(server interface{}) {
    grpcServer := server.(grpc.ServiceRegistrar)
    assetpb.RegisterOUTLETServiceServer(grpcServer, s)
}

func (s *OutletService) ListOutlet(ctx context.Context, req *assetpb.ListOutletRequest) (*assetpb.ListOutletResponse, error) {
    log.Info().Msg("List outlet")
    var outlets []*assetpb.Outlet
    var query = db
    
    // Find outlet by area ID
    if req.AreaId != 0 {
        log.Info().Msgf("List outlet with area ID: %d", req.AreaId)
        query = query.Joins("JOIN area_outlets ON outlets.outlet_id = area_outlets.outlet_id").
        Where("area_outlets.area_id = ?", req.AreaId)
    }

    result := query.Find(&outlets)
    if result.Error != nil {
        log.Error().Err(result.Error).Msg("Error fetching data")
        return &assetpb.ListOutletResponse{
            Data: nil,
            Message: "Error fetching data",
            Code: "500",
        }, nil
    }
    return &assetpb.ListOutletResponse{
        Data: outlets,
        Message: "Success",
        Code: "200",
    }, nil
}

func (s *OutletService) CreateOutlet(ctx context.Context, req *assetpb.CreateOutletRequest) (*assetpb.CreateOutletResponse, error) {
    outlet := map[string]interface{}{
        "OutletName": req.GetOutletName(),
    }

    // Create data outlet
    outletResult := db.Model(&assetpb.Outlet{}).Create(&outlet)
    if outletResult.Error != nil {
        log.Error().Err(outletResult.Error).Msg("Error creating data outlet")
        return &assetpb.CreateOutletResponse{
            Message: "Error creating data outlet",
            Code: "500",
        }, nil
    }

    // Get outlet ID
    var dataOutlet assetpb.Outlet 
    getOutletId := db.Model(&assetpb.Outlet{}).Where("outlet_name = ?", req.GetOutletName()).Find(&dataOutlet)
    if getOutletId.Error != nil {
        log.Error().Err(getOutletId.Error).Msg("Error fetching data outlet")
        return &assetpb.CreateOutletResponse{
            Message: "Error fetching data outlet",
            Code: "500",
        }, nil
    }

    areaOutlet := assetpb.AreaOutlet{
        AreaId: req.GetAreaId(),
        OutletId: dataOutlet.OutletId,
    }
    log.Info().Msgf("Outlet ID: %d", dataOutlet.OutletId)
    areaOutletResult := db.Model(&assetpb.AreaOutlet{}).Create(&areaOutlet)
    if areaOutletResult.Error != nil {
        log.Error().Err(areaOutletResult.Error).Msg("Error creating data area outlet")
        return &assetpb.CreateOutletResponse{
            Message: "Error creating data area outlet",
            Code: "500",
        }, nil
    }

    return &assetpb.CreateOutletResponse{
        Message: "Success",
        Code: "200",
    }, nil
}