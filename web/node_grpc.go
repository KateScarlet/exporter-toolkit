package web

import (
	"github.com/KateScarlet/exporter-toolkit/pb"
	"google.golang.org/grpc/metadata"
	"io"
	"os"
	"path/filepath"
)

type NodeServiceServer struct {
	pb.UnimplementedNodeServiceServer
}

func (s *NodeServiceServer) UploadFile(stream pb.NodeService_UploadFileServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return stream.SendAndClose(&pb.UploadFileStatus{
			IsSuccess: false,
			Reason:    "没有指定filename",
		})
	}
	fileNames := md["filename"]
	if len(fileNames) == 0 {
		return nil
	}
	fileName := fileNames[0]
	filePath := "/tmp/node_exporter/" + fileName
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return stream.SendAndClose(&pb.UploadFileStatus{
			IsSuccess: false,
			Reason:    err.Error(),
		})
	}
	file, err := os.Create(filePath)
	if err != nil {
		return stream.SendAndClose(&pb.UploadFileStatus{
			IsSuccess: false,
			Reason:    err.Error(),
		})
	}
	defer file.Close()

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.UploadFileStatus{
				IsSuccess: true,
				Reason:    "",
			})
		}
		if err != nil {
			return stream.SendAndClose(&pb.UploadFileStatus{
				IsSuccess: false,
				Reason:    err.Error(),
			})
		}

		_, writeErr := file.Write(chunk.Content)
		if writeErr != nil {
			return nil
		}
	}
}

//func (s *NodeServiceServer) DownloadFile(*DownloadFileRequest, pb.NodeService_DownloadFileServer) {
//
//}
