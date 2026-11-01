function Controller() {
    installer.setDefaultPageVisible(QInstaller.ComponentSelection, false);
    installer.setDefaultPageVisible(QInstaller.StartMenuSelection, false);
}
Controller.prototype.IntroductionPageCallback = function() {
    console.log("VIDOTTER_GUI_INTRO");
    gui.clickButton(buttons.NextButton, 250);
};
Controller.prototype.TargetDirectoryPageCallback = function() {
    console.log("VIDOTTER_GUI_TARGET");
    gui.clickButton(buttons.NextButton, 250);
};
Controller.prototype.ComponentSelectionPageCallback = function() {
    console.log("VIDOTTER_GUI_BAD_COMPONENT_SELECTION");
    gui.clickButton(buttons.NextButton, 250);
};
Controller.prototype.LicenseAgreementPageCallback = function() {
    console.log("VIDOTTER_GUI_LICENSE");
    var widget = gui.currentPageWidget();
    widget.AcceptLicenseCheckBox.checked = true;
    gui.clickButton(buttons.NextButton, 250);
};
Controller.prototype.ReadyForInstallationPageCallback = function() {
    console.log("VIDOTTER_GUI_READY");
    gui.clickButton(buttons.CommitButton, 250);
};
Controller.prototype.FinishedPageCallback = function() {
    console.log("VIDOTTER_GUI_FINISHED");
    var opts = gui.findChild(gui.currentPageWidget(), "openExtensionCheckBox");
    if (opts) {
        console.log("VIDOTTER_GUI_EXTENSION_OPTION_PRESENT");
        opts.checked = false;
    } else {
        console.log("VIDOTTER_GUI_EXTENSION_OPTION_MISSING");
    }
    var mcp = gui.findChild(gui.currentPageWidget(), "configureMcpCheckBox");
    if (mcp) {
        console.log("VIDOTTER_GUI_MCP_OPTION_PRESENT");
        mcp.checked = false;
    } else {
        console.log("VIDOTTER_GUI_MCP_OPTION_MISSING");
    }
    gui.clickButton(buttons.FinishButton, 350);
};
