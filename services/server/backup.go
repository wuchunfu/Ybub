package server

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
	"ybub/models"

	"github.com/pkg/sftp"
	"github.com/rs/zerolog/log"
)

const (
	// 最大并发下载数
	maxConcurrentDownloads = 5
)

// downloadTask 下载任务
type downloadTask struct {
	remotePath string
	localPath  string
}

// downloadDirectory 递归下载远程目录
func (s *SSHService) downloadDirectory(sftpClient *sftp.Client, remotePath, localPath, projectID string) error {
	// 收集所有需要下载的文件
	tasks := make([]downloadTask, 0)
	if err := s.collectDownloadTasks(sftpClient, remotePath, localPath, &tasks); err != nil {
		return err
	}

	log.Info().
		Int("fileCount", len(tasks)).
		Msg("开始多线程下载文件")

	// 创建任务通道和并发控制
	taskChan := make(chan downloadTask, len(tasks))
	errChan := make(chan error, len(tasks))
	var wg sync.WaitGroup

	// 启动工作协程池
	for i := 0; i < maxConcurrentDownloads; i++ {
		wg.Add(1)
		go s.downloadWorker(sftpClient, taskChan, errChan, &wg, projectID)
	}

	// 发送下载任务
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	// 等待所有下载完成
	wg.Wait()
	close(errChan)

	// 收集错误
	var errs []error
	for err := range errChan {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("下载过程中发生 %d 个错误: %v", len(errs), errs[0])
	}

	return nil
}

// 递归收集所有需要下载的文件
func (s *SSHService) collectDownloadTasks(sftpClient *sftp.Client, remotePath, localPath string, tasks *[]downloadTask) error {
	entries, err := sftpClient.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("读取远程目录失败 %s: %w", remotePath, err)
	}

	for _, entry := range entries {
		remoteFile := path.Join(remotePath, entry.Name())
		localFile := filepath.Join(localPath, entry.Name())

		if entry.IsDir() {
			// 创建本地子目录
			if err := os.MkdirAll(localFile, 0755); err != nil {
				return fmt.Errorf("创建本地目录失败 %s: %w", localFile, err)
			}

			// 递归收集子目录中的文件
			if err := s.collectDownloadTasks(sftpClient, remoteFile, localFile, tasks); err != nil {
				return err
			}
		} else {
			// 添加文件下载任务
			*tasks = append(*tasks, downloadTask{
				remotePath: remoteFile,
				localPath:  localFile,
			})
		}
	}

	return nil
}

// 下载工作协程
func (s *SSHService) downloadWorker(sftpClient *sftp.Client, taskChan <-chan downloadTask, errChan chan<- error, wg *sync.WaitGroup, projectID string) {
	defer wg.Done()

	for task := range taskChan {
		if err := s.downloadFile(sftpClient, task.remotePath, task.localPath, projectID); err != nil {
			log.Error().
				Err(err).
				Str("remote", task.remotePath).
				Msg("文件下载失败")
			errChan <- err
		}
	}
}

// 下载单个文件
func (s *SSHService) downloadFile(sftpClient *sftp.Client, remotePath, localPath, projectID string) error {
	log.Debug().
		Str("remote", remotePath).
		Str("local", localPath).
		Msg("下载文件")

	// 打开远程文件
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("打开远程文件失败 %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	// 创建本地文件
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败 %s: %w", localPath, err)
	}
	defer localFile.Close()

	// 复制文件内容
	written, err := io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("复制文件内容失败 %s: %w", remotePath, err)
	}

	log.Debug().
		Str("file", filepath.Base(remotePath)).
		Int64("size", written).
		Msg("文件下载完成")

	// 发送进度事件（确保 Emitter 是线程安全的）
	s.Emitter.EmitBackupTaskProgress(projectID,
		remotePath,
		written,
		fmt.Sprintf("已下载: %s (%d bytes)", filepath.Base(remotePath), written),
	)

	return nil
}

// 备份项目数据到本地 前端请勿使用
func (s *SSHService) BackupProjectData(project models.Project) error {
	log.Info().
		Str("project", project.Name).
		Str("dataPath", project.DataPath).
		Str("localDir", s.Config.BackupDir).
		Msg("开始备份项目数据")

	// 验证项目是否配置了数据路径
	if project.DataPath == "" {
		err := errors.New("项目未配置数据路径")
		log.Error().
			Str("project", project.Name).
			Msg(err.Error())
		return err
	}

	// 获取服务器配置
	server, err := s.Config.GetServer(project.ServerID)
	if err != nil {
		log.Error().
			Err(err).
			Str("project", project.Name).
			Msg("无法获取服务器配置")
		return err
	}

	// 创建 SSH 客户端
	client, err := s.getSSHClient(*server)
	if err != nil {
		log.Error().
			Err(err).
			Str("server", server.Name).
			Msg("SSH 连接失败")
		return err
	}
	defer client.Close()

	// 创建 SFTP 客户端
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		log.Error().
			Err(err).
			Msg("创建 SFTP 客户端失败")
		return err
	}
	defer sftpClient.Close()

	// 创建本地备份目录
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(s.Config.BackupDir, project.Name, timestamp)
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		log.Error().
			Err(err).
			Str("path", backupPath).
			Msg("创建本地备份目录失败")
		return err
	}

	// 发送备份开始事件
	s.Emitter.EmitBackupTaskOutput(project.ID, models.StatusStarted,
		fmt.Sprintf("开始备份 %s", project.Name),
	)

	// 递归下载远程目录
	remotePath := project.DataPath
	if err := s.downloadDirectory(sftpClient, remotePath, backupPath, project.ID); err != nil {
		log.Error().
			Err(err).
			Str("remotePath", remotePath).
			Str("localPath", backupPath).
			Msg("备份失败")
		s.Emitter.EmitBackupTaskOutput(project.ID,
			models.StatusFailed,
			fmt.Sprintf("备份失败: %v", err),
		)
		return err
	}

	log.Info().
		Str("projectID", project.ID).
		Str("project", project.Name).
		Str("backupPath", backupPath).
		Msg("备份完成")

	// 发送备份完成事件
	s.Emitter.EmitBackupTaskFinished(project.ID,
		"备份完成",
	)

	return nil
}
