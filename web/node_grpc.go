package web

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "github.com/KateScarlet/exporter-toolkit/pb"
    "google.golang.org/grpc/metadata"
    "io"
    "log"
    "os"
    "os/exec"
    "path/filepath"
)

type FileServiceServer struct {
    pb.UnimplementedFileServiceServer
}

func (s *FileServiceServer) UploadFile(stream pb.FileService_UploadFileServer) error {
    md, ok := metadata.FromIncomingContext(stream.Context())
    fileNames := md["filename"]
    if !ok || len(fileNames) == 0 {
        return stream.SendAndClose(&pb.UploadFileStatus{
            IsSuccess: false,
            Reason:    "没有在metadata里指定文件名filename",
        })
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

func (s *FileServiceServer) DownloadFile(req *pb.DownloadFileRequest, stream pb.FileService_DownloadFileServer) error {
    file, err := os.Open(req.FilePath)
    if err != nil {
        return fmt.Errorf("could not open file: %v", err)
    }
    defer file.Close()

    hash := sha256.New()
    buf := make([]byte, 1024)
    for {
        n, err := file.Read(buf)
        if err != nil && err != io.EOF {
            return fmt.Errorf("could not read file: %v", err)
        }
        if n == 0 {
            break
        }

        // Update hash
        if _, err := hash.Write(buf[:n]); err != nil {
            return fmt.Errorf("could not write to hash: %v", err)
        }

        // Send chunk with hash
        if err := stream.Send(&pb.DownloadFileStatus{
            Content: buf[:n],
            Hash:    hex.EncodeToString(hash.Sum(nil)),
        }); err != nil {
            return fmt.Errorf("could not send chunk: %v", err)
        }

        // Reset hash for next chunk
        hash.Reset()
    }

    return nil
}

type ShellServer struct {
    pb.UnimplementedShellServiceServer
}

func (s *ShellServer) StartShell(stream pb.ShellService_StartShellServer) error {
    cmd := exec.Command("bash")

    stdin, err := cmd.StdinPipe()
    if err != nil {
        return err
    }
    defer stdin.Close()

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return err
    }
    defer stdout.Close()

    if err := cmd.Start(); err != nil {
        return err
    }

    go func() {
        buf := make([]byte, 1024)
        for {
            n, err := stdout.Read(buf)
            if err != nil {
                return
            }
            if err := stream.Send(&pb.CommandResponse{Output: string(buf[:n])}); err != nil {
                log.Printf("Failed to send data over stream: %v", err)
                stdout.Close()
                return
            }
        }
    }()

    go func() {
        for {
            req, err := stream.Recv()
            if err != nil {
                stdin.Close()
                return
            }
            input := req.GetCommand() + "\n"
            stdin.Write([]byte(input))
        }
    }()

    if err := cmd.Wait(); err != nil {
        return err
    }

    return nil
}
