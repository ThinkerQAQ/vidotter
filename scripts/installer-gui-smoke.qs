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
    var note = gui.findChild(gui.currentPageWidget(), "requiredExtensionNote");
    if (note) {
        console.log("VIDOTTER_GUI_MANDATORY_GUIDE_PRESENT");
    } else {
        console.log("VIDOTTER_GUI_MANDATORY_GUIDE_MISSING");
    }
    var optional = gui.findChild(gui.currentPageWidget(), "openExtensionCheckBox");
    if (optional) console.log("VIDOTTER_GUI_BAD_OPTIONAL_EXTENSION");
    var mcp = gui.findChild(gui.currentPageWidget(), "configureMcpCheckBox");
    if (mcp) console.log("VIDOTTER_GUI_BAD_MCP_OPTION");
    gui.clickButton(buttons.FinishButton, 350);
};
