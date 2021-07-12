// +build windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package mount

// Simple wrappers around SetVolumeMountPoint and DeleteVolumeMountPoint

import (
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// Mount volumePath (in format '\\?\Volume{GUID}' at targetPath.
// https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-setvolumemountpointw
func setVolumeMountPoint(targetPath string, volumePath string) error {
	if !strings.HasPrefix(volumePath, "\\\\?\\Volume{") {
		return errors.Errorf("unable to mount non-volume path %s", volumePath)
	}

	// Both must end in a backslash
	slashedTarget := filepath.Clean(targetPath) + string(filepath.Separator)
	slashedVolume := volumePath + string(filepath.Separator)

	targetP, err := syscall.UTF16PtrFromString(slashedTarget)
	if err != nil {
		return errors.Wrapf(err, "unable to utf16-ise %s", slashedTarget)
	}

	volumeP, err := syscall.UTF16PtrFromString(slashedVolume)
	if err != nil {
		return errors.Wrapf(err, "unable to utf16-ise %s", slashedVolume)
	}

	if err := windows.SetVolumeMountPoint(targetP, volumeP); err != nil {
		return errors.Wrapf(err, "failed calling SetVolumeMount('%s', '%s')", slashedTarget, slashedVolume)
	}

	return nil
}

// Remove the volume mount at targetPath
// https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-deletevolumemountpointa
func deleteVolumeMountPoint(targetPath string) error {
	// Must end in a backslash
	slashedTarget := filepath.Clean(targetPath) + string(filepath.Separator)

	targetP, err := syscall.UTF16PtrFromString(slashedTarget)
	if err != nil {
		return errors.Wrapf(err, "unable to utf16-ise %s", slashedTarget)
	}

	volumeName, err := getVolumeNameForVolumeMountPoint(targetPath)
	if err != nil {
		return errors.Wrapf(err, "failed calling getVolumeNameForVolumeMountPoint('%s')", targetPath)
	}

	if err := windows.DeleteVolumeMountPoint(targetP); err != nil {
		return errors.Wrapf(err, "failed calling DeleteVolumeMountPoint('%s')", slashedTarget)
	}

	// Strip the trailing slash off for CreaetFile.
	if volumeName[len(volumeName)-1] == filepath.Separator {
		volumeName = volumeName[:len(volumeName)-1]
	}

	volumeNameP, err := syscall.UTF16PtrFromString(volumeName)
	if err != nil {
		return errors.Wrapf(err, "unable to utf16-ise %s", volumeName)
	}

	volumeHandle, err := windows.CreateFile(volumeNameP, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return errors.Wrapf(err, "failed opening volumeHandle: %s", volumeName)
	}
	defer windows.CloseHandle(volumeHandle)

	if err := windows.FlushFileBuffers(volumeHandle); err != nil {
		return errors.Wrapf(err, "failed flushing volumeHandle")
	}

	return nil
}

// getVolumeNameForVolumeMountPoint returns a volume path (in format '\\?\Volume{GUID}'
// for the volume mounted at targetPath.
// https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumenameforvolumemountpointw
func getVolumeNameForVolumeMountPoint(targetPath string) (string, error) {
	// Must end in a backslash
	slashedTarget := filepath.Clean(targetPath)
	if slashedTarget[len(slashedTarget)-1] != filepath.Separator {
		slashedTarget = slashedTarget + string(filepath.Separator)
	}

	targetP, err := windows.UTF16PtrFromString(slashedTarget)
	if err != nil {
		return "", errors.Wrapf(err, "unable to utf16-ise %s", slashedTarget)
	}

	bufferlength := uint32(50) // "A reasonable size for the buffer" per the documentation.
	buffer := make([]uint16, bufferlength)

	if err := windows.GetVolumeNameForVolumeMountPoint(targetP, &buffer[0], bufferlength); err != nil {
		return "", errors.Wrapf(err, "failed calling GetVolumeNameForVolumeMountPoint('%s', ..., %d)", slashedTarget, bufferlength)
	}

	return windows.UTF16ToString(buffer), nil
}
