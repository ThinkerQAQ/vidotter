// Windows GUI launchers adapted from RemoteBrowserMCP PR #17.
// Compiled as WinExe with the VidOtter icon; no PowerShell window or credentials on argv.
using System;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Net;
using System.Reflection;
using System.Text.RegularExpressions;
using System.Threading;
using System.Windows.Forms;

internal static class VidOtterLauncher {
    private static string Root {
        get {
            string folder = Path.GetDirectoryName(Assembly.GetExecutingAssembly().Location);
#if MAINTENANCE
            return Path.GetFullPath(folder);
#else
            return Path.GetFullPath(Path.Combine(folder, ".."));
#endif
        }
    }
    private static string BridgeBinary {
        get { return Path.Combine(Root, "bin", "vidotter-windows-amd64.exe"); }
    }
    private static string BaseURL {
        get {
            string defaultURL = "http://127.0.0.1:17891";
            string state = Path.Combine(Root, "Data", "bridge.json");
            try {
                if (!File.Exists(state)) return defaultURL;
                // Read the loopback URL only. Never display or log the private token.
                Match m = Regex.Match(File.ReadAllText(state),
                    "\"baseUrl\"\\s*:\\s*\"(?<url>http://127\\.0\\.0\\.1:[0-9]{2,5})\"");
                Uri url;
                if (m.Success && Uri.TryCreate(m.Groups["url"].Value, UriKind.Absolute, out url)
                    && url.Host == "127.0.0.1" && url.Port > 0) return url.GetLeftPart(UriPartial.Authority);
            } catch { }
            return defaultURL;
        }
    }
    private static bool PageAvailable(string baseURL) {
        try {
            HttpWebRequest request = (HttpWebRequest)WebRequest.Create(
                baseURL + "/console/popup/popup.html");
            request.Method = "GET";
            request.Timeout = 1200;
            request.Proxy = null;
            using (HttpWebResponse response = (HttpWebResponse)request.GetResponse())
                return response.StatusCode == HttpStatusCode.OK;
        } catch { return false; }
    }
    private static void OpenWebSettings() {
        if (!File.Exists(BridgeBinary))
            throw new FileNotFoundException("VidOtter Bridge is not installed.", BridgeBinary);
        string baseURL = BaseURL;
        if (!PageAvailable(baseURL)) {
            ProcessStartInfo start = new ProcessStartInfo(BridgeBinary, "--bridge-service");
            start.WorkingDirectory = Root;
            start.CreateNoWindow = true;
            start.UseShellExecute = false;
            start.WindowStyle = ProcessWindowStyle.Hidden;
            Process.Start(start);
            for (int i = 0; i < 30; i++) {
                Thread.Sleep(200);
                baseURL = BaseURL;
                if (PageAvailable(baseURL)) break;
            }
            if (!PageAvailable(baseURL))
                throw new InvalidOperationException(
                    "VidOtter Bridge did not become ready. Check the installed runtime.");
        }
        ProcessStartInfo browser = new ProcessStartInfo(baseURL + "/console/popup/popup.html");
        browser.UseShellExecute = true;
        Process.Start(browser);
    }
    private static void OpenGuide() {
        string file = Path.Combine(Root, "README_ZH.md");
        if (!File.Exists(file)) file = Path.Combine(Root, "README.md");
        if (!File.Exists(file)) throw new FileNotFoundException("VidOtter local guide was not installed.");
        Process.Start(new ProcessStartInfo(file) { UseShellExecute = true });
    }
    private static void LaunchMaintenance() {
        string file = Path.Combine(Root, "VidOtter-Maintenance.exe");
        if (!File.Exists(file)) throw new FileNotFoundException("VidOtter maintenance tool is missing.", file);
        Process.Start(new ProcessStartInfo(file) { WorkingDirectory = Root, UseShellExecute = true });
    }
#if MAINTENANCE
    private static Form ManagementWindow() {
        Form form = new Form();
        form.Text = "VidOtter - Manage";
        form.StartPosition = FormStartPosition.CenterScreen;
        form.Size = new Size(470, 260);
        form.MinimumSize = form.Size;
        form.MaximumSize = form.Size;
        form.MaximizeBox = false;
        string iconFile = Path.Combine(Root, "assets", "vidotter.ico");
        if (File.Exists(iconFile)) form.Icon = new Icon(iconFile);
        Label title = new Label();
        title.Text = "Manage VidOtter";
        title.Location = new Point(25, 20);
        title.Size = new Size(420, 28);
        title.Font = new Font(title.Font, FontStyle.Bold);
        form.Controls.Add(title);
        Label explanation = new Label();
        explanation.Text = "Open settings, update VidOtter or uninstall. Your local data remains private.";
        explanation.Location = new Point(25, 55);
        explanation.Size = new Size(420, 45);
        form.Controls.Add(explanation);
        Button configure = new Button();
        configure.Text = "Open VidOtter Settings";
        configure.Location = new Point(25, 115);
        configure.Size = new Size(255, 36);
        configure.Click += (sender, args) => {
            try { OpenWebSettings(); }
            catch(Exception error) { MessageBox.Show(error.Message, form.Text, MessageBoxButtons.OK, MessageBoxIcon.Error); }
        };
        form.Controls.Add(configure);
        Button uninstall = new Button();
        uninstall.Text = "Manage / Uninstall";
        uninstall.Location = new Point(290, 115);
        uninstall.Size = new Size(150, 36);
        uninstall.Click += (sender, args) => {
            DialogResult decision = MessageBox.Show(
                "Open the Qt maintenance tool?\nInstalled files may be removed, but VidOtter Data is retained.",
                "VidOtter", MessageBoxButtons.YesNo, MessageBoxIcon.Question);
            if (decision == DialogResult.Yes) {
                try { LaunchMaintenance(); form.Close(); }
                catch(Exception error) { MessageBox.Show(error.Message, form.Text, MessageBoxButtons.OK, MessageBoxIcon.Error); }
            }
        };
        form.Controls.Add(uninstall);
        Label hint = new Label();
        hint.Text = "Changing Tunnel credentials never requires reinstalling VidOtter.";
        hint.Location = new Point(25, 169);
        hint.Size = new Size(420, 32);
        form.Controls.Add(hint);
        return form;
    }
#endif
    [STAThread]
    private static int Main(string[] args) {
        try {
            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);
            if (args.Length == 1 && args[0] == "--self-test") {
#if MAINTENANCE
                using (Form form = ManagementWindow()) {
                    if (form.Icon == null) return 7;
                    int buttons = 0;
                    foreach (Control c in form.Controls) if (c is Button) buttons++;
                    return buttons == 2 ? 0 : 8;
                }
#else
                return File.Exists(BridgeBinary) ? 0 : 7;
#endif
            }
#if MAINTENANCE
            Application.Run(ManagementWindow());
#else
            if (args.Length == 1 && args[0] == "--guide") OpenGuide();
            else if (args.Length == 0) OpenWebSettings();
            else throw new ArgumentException("Unsupported VidOtter launcher action");
#endif
            return 0;
        } catch (Exception error) {
            MessageBox.Show(error.Message, "VidOtter", MessageBoxButtons.OK, MessageBoxIcon.Error);
            return 1;
        }
    }
}
