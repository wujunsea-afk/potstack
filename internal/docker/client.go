package docker

import (
	"bytes"
	"fmt"
	"os/exec"
)

// PullAndTag 拉取远程镜像并打本地 Tag
func PullAndTag(remoteImage, localTag string) error {
	// 拉取
	pullCmd := exec.Command("docker", "pull", remoteImage)
	var stderr bytes.Buffer
	pullCmd.Stderr = &stderr
	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("docker pull %s failed: %w, stderr: %s", remoteImage, err, stderr.String())
	}

	// 打 Tag
	tagCmd := exec.Command("docker", "tag", remoteImage, localTag)
	tagCmd.Stderr = &stderr
	if err := tagCmd.Run(); err != nil {
		return fmt.Errorf("docker tag failed: %w, stderr: %s", err, stderr.String())
	}
	return nil
}

// RemoveTag 删除本地 Tag
func RemoveTag(localTag string) error {
	return exec.Command("docker", "rmi", localTag).Run()
}

// ImageExists 检查本地镜像是否存在
func ImageExists(tag string) bool {
	return exec.Command("docker", "image", "inspect", tag).Run() == nil
}
