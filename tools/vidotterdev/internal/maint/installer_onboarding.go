package maint

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/controller.qs assets/first-run.ui assets/first-run.ps1 assets/Open-VidOtter-Settings.sh
var onboardingAssets embed.FS

func writeInstallerAsset(from, to string) error {
	b, err := onboardingAssets.ReadFile("assets/" + from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}

// GUI completion offers an unchecked option to open the local README.
// Headless installs and unchecked GUI installs never launch another application.
func writeInstallerOnboarding(dir, meta, payload string, targetOS string) error {
	for _, file := range []struct{ asset, path string }{
		{"controller.qs", filepath.Join(dir, "config", "controller.qs")},
		{"first-run.ui", filepath.Join(meta, "first-run.ui")},
	} {
		if err := writeInstallerAsset(file.asset, file.path); err != nil {
			return err
		}
	}
	if targetOS == "windows" {
		if err := writeInstallerAsset("first-run.ps1", filepath.Join(payload, "scripts", "VidOtter-FirstRun.ps1")); err != nil {
			return err
		}
	} else {
		script := filepath.Join(payload, "scripts", "Open-VidOtter-Settings.sh")
		if err := writeInstallerAsset("Open-VidOtter-Settings.sh", script); err != nil {
			return err
		}
		if err := os.Chmod(script, 0o755); err != nil {
			return err
		}
	}
	return nil
}

const firstRunQtHandlers = `
Component.prototype.showFirstRunOptions = function() {
    try {
        if (installer.isInstaller() && !installer.isCommandLineInstance() &&
            installer.status == QInstaller.Success) {
            installer.addWizardPageItem(component, "VidOtterFirstRun", QInstaller.InstallationFinished);
        }
    } catch (e) { console.log("VidOtter first-run options: " + e); }
};
Component.prototype.finishFirstRun = function() {
    try {
        if (!installer.isInstaller() || installer.isCommandLineInstance() ||
            installer.status != QInstaller.Success) return;
        var form = component.userInterface("VidOtterFirstRun");
        if (!form) return;
        var guide = form.openGuideCheckBox && form.openGuideCheckBox.checked;
        var configure = form.configureNowCheckBox && form.configureNowCheckBox.checked;
        if (!guide && !configure) return;
        var target = installer.value("TargetDir");
        if ("__VIDOTTER_PLATFORM__" == "windows") {
            var launcher = installer.toNativeSeparators(target + "/bin/VidOtter-Configure.exe");
            if (guide) {
                console.log("VIDOTTER_LOCAL_GUIDE_REQUESTED");
                installer.executeDetached(launcher, ["--guide"], target);
            }
            if (configure) {
                console.log("VIDOTTER_LOCAL_CONFIGURATION_REQUESTED");
                installer.executeDetached(launcher, [], target);
            }
        } else {
            if (guide) {
                console.log("VIDOTTER_LOCAL_GUIDE_REQUESTED");
                QDesktopServices.openUrl("file:///" + target + "/README.md");
            }
            if (configure) {
                console.log("VIDOTTER_LOCAL_CONFIGURATION_REQUESTED");
                installer.executeDetached("/bin/sh", [target + "/scripts/Open-VidOtter-Settings.sh"], target);
            }
        }
    } catch (e) { console.log("VidOtter setup helper: " + e); }
};
`

func firstRunComponentScript(bridgeInstaller, shortcutOperations, targetOS string) string {
	return fmt.Sprintf(`function Component() {
    if (installer.isInstaller() && !installer.isCommandLineInstance()) {
        installer.installationFinished.connect(this, Component.prototype.showFirstRunOptions);
        installer.finishButtonClicked.connect(this, Component.prototype.finishFirstRun);
    }
}

Component.prototype.createOperations = function() {
    component.createOperations();
    component.addOperation("Execute",
        "@TargetDir@/bin/%s", "install", "--root", "@TargetDir@",
        "UNDOEXECUTE", "@TargetDir@/bin/%s", "uninstall", "--root", "@TargetDir@");
%s
}
%s
`, bridgeInstaller, bridgeInstaller, shortcutOperations, strings.ReplaceAll(firstRunQtHandlers, "__VIDOTTER_PLATFORM__", targetOS))
}
