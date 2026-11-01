package maint

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/controller.qs assets/first-run.ui assets/first-run.ps1
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

// The first-run browser activation guide opens on every successful GUI install;
// headless installs never launch a browser.
// Windows ships an explicit helper which opens the browser extensions page
// and Explorer at the precise path. It never silently installs the extension.
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
        console.log("VIDOTTER_EXTENSION_GUIDE_REQUIRED");
        var target = installer.value("TargetDir");
        if ("__VIDOTTER_PLATFORM__" == "windows") {
            var script = installer.toNativeSeparators(target + "/scripts/VidOtter-FirstRun.ps1");
            installer.executeDetached("powershell.exe",
                ["-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden",
                 "-File", script, "-Mode", "extension"], target);

        } else {
            QDesktopServices.openUrl("file:///" + target + "/extension");
            QDesktopServices.openUrl("file:///" + target + "/README.md");

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
