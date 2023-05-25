package bootstrap

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/common"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

//go:embed templates/gardener-node-agent.service.tpl
var systemdUnit string

func Bootstrap(ctx context.Context, log logr.Logger) error {
	log.Info("bootstrap")

	err := renderSystemdUnit()
	if err != nil {
		return fmt.Errorf("unable to render system unit %w", err)
	}

	err = dbus.Enable(ctx, v1alpha1.NodeAgentUnitName)
	if err != nil {
		return fmt.Errorf("unable to enable system unit %w", err)
	}

	err = dbus.Start(ctx, nil, nil, v1alpha1.NodeAgentUnitName)
	if err != nil {
		return fmt.Errorf("unable to start system unit %w", err)
	}

	// Disable gardener-node-init unit
	err = dbus.Disable(ctx, v1alpha1.NodeInitUnitName)
	if err != nil {
		return fmt.Errorf("unable to disable system unit %w", err)
	}

	err = cleanupCCD(ctx)
	if err != nil {
		return fmt.Errorf("unable to cleanup cloud-config-downloader: %w", err)
	}

	err = formatDataDevice(log)
	if err != nil {
		return err
	}

	// Stop itself, must be the last action because it will not get executed anyway.
	err = dbus.Stop(ctx, nil, nil, v1alpha1.NodeInitUnitName)

	return err
}

func renderSystemdUnit() error {
	tpl := template.Must(
		template.New("v4").
			Funcs(template.FuncMap{"StringsJoin": strings.Join}).
			Parse(systemdUnit),
	)

	var target bytes.Buffer
	// TODO do we need data for the template
	err := tpl.Execute(&target, nil)
	if err != nil {
		return err
	}

	return os.WriteFile(path.Join("/etc/systemd/system/", v1alpha1.NodeAgentUnitName), target.Bytes(), 0600)
}

func cleanupCCD(ctx context.Context) error {
	if _, err := os.Stat(path.Join("/etc/systemd/system/", downloader.UnitName)); err != nil && os.IsNotExist(err) {
		return nil
	}

	err := dbus.Stop(ctx, nil, nil, downloader.UnitName)
	if err != nil {
		return fmt.Errorf("unable to stop system unit %w", err)
	}

	err = dbus.Disable(ctx, downloader.UnitName)
	if err != nil {
		return fmt.Errorf("unable to disable system unit %w", err)
	}

	return nil
}

func formatDataDevice(log logr.Logger) error {
	config, err := common.ReadNodeAgentConfiguration()
	if err != nil {
		return err
	}

	if config.KubeletDataVolumeSize == nil {
		return nil
	}

	size := *config.KubeletDataVolumeSize

	label := "KUBEDEV"
	out, err := execCommand("blkid", "--label", "label="+label)
	if err != nil {
		return fmt.Errorf("unable to execute blkid output:%s %w", out, err)
	}
	if out != nil {
		log.Info("kubernetes datavolume already mounted", "blkid output", string(out))
		return nil
	}

	out, err = execCommand("lsblk", "-dbsnP", "-o", "NAME,PARTTYPE,MOUNTPOINT,FSTYPE,SIZE,PATH,TYPE")
	if err != nil {
		return fmt.Errorf("unable to execute lsblk output:%s %w", out, err)
	}

	targetDevice := ""
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "TYPE=\"disk\"") {
			continue
		}
		if strings.Contains(line, " MOUNTPOINT=\"/") {
			continue
		}
		if strings.Contains(line, " SIZE="+strconv.FormatInt(size, 10)) {
			var found bool
			targetDevice, _, found = strings.Cut(line, ":")
			if !found {
				continue
			}
		}
	}

	if targetDevice == "" {
		log.Info("no kubernetes datavolume with matching size found", "size", size)
		return nil
	}
	log.Info("kubernetes datavolume with matching size found", "device", targetDevice, "size", size)

	out, err = execCommand("mkfs.ext4", "-L", label, "-O", "quota", "-E", "lazy_itable_init=0,lazy_journal_init=0,quotatype=usrquota:grpquota:prjquota", "/dev/"+targetDevice)
	if err != nil {
		return fmt.Errorf("unable to execute mkfs output:%s %w", out, err)
	}

	err = os.MkdirAll("/tmp/varlibcp", fs.ModeDir)
	if err != nil {
		return fmt.Errorf("unable to create temporary mount dir %w", err)
	}

	out, err = execCommand("mount", "LABEL="+label, "/tmp/varlibcp")
	if err != nil {
		return fmt.Errorf("unable to execute mkfs output:%s %w", out, err)
	}

	out, err = execCommand("cp", "-r", "/var/lib/*", "/tmp/varlibcp")
	if err != nil {
		return fmt.Errorf("unable to copy output:%s %w", out, err)
	}

	out, err = execCommand("umount", "/tmp/varlibcp")
	if err != nil {
		return fmt.Errorf("unable to execute umount output:%s %w", out, err)
	}
	out, err = execCommand("mount", "LABEL="+label, "/var/lib", "-o", "defaults,prjquota,errors=remount-ro")
	if err != nil {
		return fmt.Errorf("unable to execute mount output:%s %w", out, err)
	}

	log.Info("kubernetes datavolume mounted to /var/lib", "device", targetDevice, "size", size)

	// TODO: implement kubelet-data-volume feature:
	// function format-data-device() {
	// 	LABEL=KUBEDEV
	// 	if ! blkid --label $LABEL >/dev/null; then
	// 	  DISK_DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,MOUNTPOINT,FSTYPE,SIZE,PATH,TYPE | grep 'TYPE="disk"')
	// 	  while IFS= read -r line; do
	// 		MATCHING_DEVICE_CANDIDATE=$(echo "$line" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="{{ .kubeletDataVolume.size }}"')
	// 		if [ -z "$MATCHING_DEVICE_CANDIDATE" ]; then
	// 		  continue
	// 		fi

	// 		# Skip device if it's already mounted.
	// 		DEVICE_NAME=$(echo "$MATCHING_DEVICE_CANDIDATE" | cut -f2 -d\")
	// 		DEVICE_MOUNTS=$(lsblk -dbsnP -o NAME,MOUNTPOINT,TYPE | grep -e "^NAME=\"$DEVICE_NAME.*\".*TYPE=\"part\"$")
	// 		if echo "$DEVICE_MOUNTS" | awk '{print $2}' | grep "MOUNTPOINT=\"\/.*\"" > /dev/null; then
	// 		  continue
	// 		fi

	// 		TARGET_DEVICE_NAME="$DEVICE_NAME"
	// 		break
	// 	  done <<< "$DISK_DEVICES"

	// 	  if [ -z "$TARGET_DEVICE_NAME" ]; then
	// 		echo "No kubelet data device found"
	// 		return
	// 	  fi

	// 	  echo "Matching kubelet data device by size : {{ .kubeletDataVolume.size }}"
	// 	  echo "detected kubelet data device $TARGET_DEVICE_NAME"
	// 	  mkfs.ext4 -L $LABEL -O quota -E lazy_itable_init=0,lazy_journal_init=0,quotatype=usrquota:grpquota:prjquota  /dev/$TARGET_DEVICE_NAME
	// 	  echo "formatted and labeled data device $TARGET_DEVICE_NAME"
	// 	  mkdir /tmp/varlibcp
	// 	  mount LABEL=$LABEL /tmp/varlibcp
	// 	  echo "mounted temp copy dir on data device $TARGET_DEVICE_NAME"
	// 	  cp -a /var/lib/* /tmp/varlibcp/
	// 	  umount /tmp/varlibcp
	// 	  echo "copied /var/lib to data device $TARGET_DEVICE_NAME"
	// 	  mount LABEL=$LABEL /var/lib -o defaults,prjquota,errors=remount-ro
	// 	  echo "mounted /var/lib on data device $TARGET_DEVICE_NAME"
	// 	fi
	//   }

	//   format-data-device

	return nil
}

func execCommand(name string, arg ...string) ([]byte, error) {
	cmd, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("unable to locate program:%q in path %w", name, err)
	}

	out, err := exec.Command(cmd, arg...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unable to execute %q output:%s %w", name, out, err)
	}
	return out, nil
}
