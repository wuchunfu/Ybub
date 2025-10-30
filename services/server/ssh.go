package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"ybub/models"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

// 测试服务器连接
func (s *SSHService) TestConnection(server models.Server) error {
	log.Info().
		Str("server", server.Name).
		Str("host", server.Host).
		Msg("开始测试 SSH 连接")

	client, err := s.getSSHClient(server)
	if err != nil {
		log.Error().
			Err(err).
			Str("server", server.Name).
			Msg("SSH 连接失败")
		return err
	}
	defer client.Close()

	log.Info().
		Str("server", server.Name).
		Msg("SSH 连接成功")
	return nil
}
func (s *SSHService) ExecCommand(server models.Server, commandID string, commands []string, cwd string) error {
	log.Info().
		Str("server", server.Name).
		Str("id", commandID).
		Str("cwd", cwd).
		Strs("commands", commands).
		Msg("开始执行远程命令")

	client, err := s.getSSHClient(server)
	if err != nil {
		log.Error().
			Err(err).
			Str("server", server.Name).
			Msg("SSH 连接创建失败")
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Error().Err(err).Msg("创建 SSH Session 失败")
		return err
	}
	defer session.Close()

	cmdStr := strings.Join(commands, " && ")
	if cwd != "" {
		cmdStr = fmt.Sprintf("cd %s && %s", cwd, cmdStr)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return err
	}

	if err := session.Start(cmdStr); err != nil {
		return err
	}

	// === 创建中断通道并存入 map ===
	cancelCh := make(chan struct{})
	s.cancelMap.Store(commandID, cancelCh)
	defer s.cancelMap.Delete(commandID)

	// 实时读取 stdout/stderr 并通过事件发前端
	done := make(chan struct{})
	go func() {
		s.streamOutput(stdout, func(line string) {
			// !临时调试
			// log.Debug().
			// 	Str("server", server.Name).
			// 	Str("id", commandID).
			// 	Msgf("[STDOUT] %s", line)
			s.Emitter.EmitSshOutput(commandID, "stdout", line)
		})
		close(done)
	}()
	go s.streamOutput(stderr, func(line string) {
		log.Warn().
			Str("server", server.Name).
			Str("id", commandID).
			Msgf("[STDERR] %s", line)
		s.Emitter.EmitSshOutput(commandID, "stderr", line)
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.Wait()
	}()

	select {
	case <-cancelCh:
		log.Warn().
			Str("server", server.Name).
			Str("id", commandID).
			Msg("收到退出指令，终止远程命令执行")
		_ = session.Signal(ssh.SIGKILL)
		_ = session.Close()
		s.Emitter.EmitSshOutput(commandID, "cancelled", "")
		return fmt.Errorf("命令被中断")

	case err := <-errCh:
		if err != nil {
			log.Error().
				Err(err).
				Str("server", server.Name).
				Msg("SSH 命令执行出错")
			s.Emitter.EmitSshOutput(commandID, "error", "")
			return err
		}
	}

	<-done
	log.Info().
		Str("server", server.Name).
		Str("id", commandID).
		Msg("命令执行完成")
	s.Emitter.EmitSshCompleted(commandID)
	return nil
}

// ExecProjectCommand 用于 DockerProject，复用通用 ExecCommand
func (s *SSHService) ExecProjectCommand(project models.Project, commandID string, commands []string) error {
	server, err := s.Config.GetServer(project.ServerID)
	if err != nil {
		log.Error().
			Err(err).
			Str("project", project.Name).
			Msg("无法获取服务器配置")
		return err
	}
	return s.ExecCommand(*server, commandID, commands, project.Path)
}

func (s *SSHService) StopCommand(commandID string) {
	if ch, ok := s.cancelMap.Load(commandID); ok {
		close(ch.(chan struct{}))
		s.cancelMap.Delete(commandID)
		log.Info().
			Str("id", commandID).
			Msg("发送中断信号成功")
	} else {
		log.Warn().
			Str("id", commandID).
			Msg("未找到可中断的命令")
	}
}

// 用服务器 ID 执行单条命令
func (s *SSHService) ExecCommandByServerID(serverID, commandID, command string) error {
	log.Info().
		Str("serverID", serverID).
		Str("commandID", commandID).
		Str("command", command).
		Msg("执行单条 SSH 命令")

	server, err := s.Config.GetServer(serverID)
	if err != nil {
		log.Warn().
			Str("serverID", serverID).
			Msg("未找到服务器")
		return err
	}
	return s.ExecCommand(*server, commandID, []string{command}, "")
}

// 创建 SSH Client，支持密码和私钥（自动展开 ~）
func (s *SSHService) getSSHClient(server models.Server) (*ssh.Client, error) {
	log.Debug().
		Str("server", server.Name).
		Str("host", server.Host).
		Int("port", server.Port).
		Msg("尝试建立 SSH 连接")

	config := &ssh.ClientConfig{
		User:            server.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if server.IdentityFile != "" {
		identityPath := server.IdentityFile
		if len(identityPath) >= 2 && identityPath[:2] == "~/" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			identityPath = filepath.Join(home, identityPath[2:])
		}
		key, err := os.ReadFile(identityPath)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		log.Debug().Msg("使用私钥认证连接 SSH")
	} else if server.Password != "" {
		config.Auth = []ssh.AuthMethod{ssh.Password(server.Password)}
		log.Debug().Msg("使用密码认证连接 SSH")
	} else {
		return nil, errors.New("必须提供密码或私钥")
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", server.Host, server.Port), config)
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("server", server.Name).
		Msg("SSH 连接成功")
	return client, nil
}

// streamOutput 实时读取输出
func (s *SSHService) streamOutput(r io.Reader, callback func(string)) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		callback(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Error().Err(err).Msg("读取 SSH 输出时发生错误")
	}
}
