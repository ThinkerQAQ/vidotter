function Controller() {
    if (installer.isInstaller() && !installer.isCommandLineInstance()) {
        // One product; no component-selection or Start Menu selection screens.
        installer.setDefaultPageVisible(QInstaller.ComponentSelection, false);
        installer.setDefaultPageVisible(QInstaller.StartMenuSelection, false);
    }
}
Controller.prototype.IntroductionPageCallback = function() {
    if (!installer.isInstaller()) return;
    var page = gui.currentPageWidget();
    if (page && page.MessageLabel) {
        page.MessageLabel.setText("Install VidOtter. The local downloader and MCP server are included.\nAfter installation, follow the browser extension activation guide.");
    }
};
