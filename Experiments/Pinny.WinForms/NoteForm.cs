using System.ComponentModel;
using System.Runtime.InteropServices;
using Pinny;

namespace Pinny.WinFormsVariant;

internal sealed class NoteForm : Form
{
    private const int WmNcHitTest = 0x84, WmNcLButtonDown = 0xA1, HtCaption = 2;
    [DllImport("user32.dll")] private static extern bool ReleaseCapture();
    [DllImport("user32.dll")] private static extern IntPtr SendMessage(IntPtr window, int message, IntPtr wParam, IntPtr lParam);

    private readonly NotesContext _context;
    private readonly Guid _id;
    private readonly Panel _header = new() { Dock = DockStyle.Top, Height = 32 };
    private readonly TextBox _text = new() { Multiline = true, AcceptsReturn = true, AcceptsTab = true,
        WordWrap = true, ScrollBars = ScrollBars.Vertical, BorderStyle = BorderStyle.None,
        Dock = DockStyle.Fill, MaxLength = int.MaxValue };
    private readonly Button _new = Button("+", 34), _menu = Button("Menu", 45),
        _pin = Button("◇", 36), _close = Button("×", 36);
    private readonly Label _title = new() { Text = "Pinny", Dock = DockStyle.Fill,
        TextAlign = ContentAlignment.MiddleLeft, Padding = new Padding(10, 0, 0, 0) };
    private readonly ContextMenuStrip _menuItems = new();
    private bool _restoring = true;
    private string _theme = "Light";

    private static Button Button(string text, int width) => new()
    { Text = text, Width = width, Dock = DockStyle.Right, FlatStyle = FlatStyle.Flat };

    public NoteForm(NotesContext context, NoteState state)
    {
        _context = context; _id = state.Id;
        Text = "Pinny WinForms";
        Icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath);
        FormBorderStyle = FormBorderStyle.None;
        MinimumSize = new Size(210, 120);
        StartPosition = FormStartPosition.Manual;
        var content = new Panel { Dock = DockStyle.Fill, Padding = new Padding(10, 8, 10, 8) };
        content.Controls.Add(_text);
        Controls.Add(content); Controls.Add(_header);
        _header.Controls.AddRange([_title, _new, _menu, _pin, _close]);
        _title.MouseDown += (_, e) =>
        {
            if (e.Button == MouseButtons.Left)
            {
                ReleaseCapture();
                SendMessage(Handle, WmNcLButtonDown, (IntPtr)HtCaption, IntPtr.Zero);
            }
        };
        _new.Click += (_, _) => _context.NewNote(this);
        _close.Click += (_, _) => Close();
        _pin.Click += (_, _) => { TopMost = !TopMost; _pin.Text = TopMost ? "◆" : "◇"; _context.QueueSave(); };
        _menu.Click += (_, _) => _menuItems.Show(_menu, 0, _menu.Height);
        foreach (string theme in new[] { "Light", "Dark", "Paper" })
        {
            string selected = theme;
            _menuItems.Items.Add(theme, null, (_, _) => { ApplyTheme(selected); _context.QueueSave(); _text.Focus(); });
        }
        _menuItems.Items.Add(new ToolStripSeparator());
        _menuItems.Items.Add("Quit Pinny", null, (_, _) => _context.Quit());
        foreach (Button button in new[] { _new, _menu, _pin, _close }) button.FlatAppearance.BorderSize = 0;

        ApplyTheme(state.Theme);
        _text.Text = state.Text ?? string.Empty;
        Size = new Size((int)Math.Clamp(double.IsFinite(state.Width) ? state.Width : 320, 210, 10000),
            (int)Math.Clamp(double.IsFinite(state.Height) ? state.Height : 320, 120, 10000));
        Point desired = new(double.IsFinite(state.Left) ? (int)state.Left : 0,
            double.IsFinite(state.Top) ? (int)state.Top : 0);
        Rectangle bounds = Screen.FromPoint(desired).WorkingArea;
        Location = double.IsFinite(state.Left) && double.IsFinite(state.Top)
            ? new Point(Math.Clamp(desired.X, bounds.Left - Width + 48, bounds.Right - 48),
                Math.Clamp(desired.Y, bounds.Top, bounds.Bottom - 48))
            : new Point(bounds.Left + (bounds.Width - Width) / 2, bounds.Top + (bounds.Height - Height) / 2);
        _pin.Text = state.IsPinned ? "◆" : "◇";
        _restoring = false;
        _text.TextChanged += (_, _) => Changed();
        Move += (_, _) => Changed(); Resize += (_, _) => Changed();
        Shown += (_, _) => { TopMost = state.IsPinned; _text.Focus(); };
    }

    private void Changed() { if (!_restoring) _context.QueueSave(); }

    public NoteState CaptureState() => new()
    {
        Id = _id, Text = _text.Text, Left = Left, Top = Top,
        Width = Width, Height = Height, IsPinned = TopMost, Theme = _theme
    };

    private void ApplyTheme(string? theme)
    {
        _theme = theme is "Dark" or "Paper" ? theme : "Light";
        (Color surface, Color header, Color ink) = _theme switch
        {
            "Dark" => (Color.FromArgb(36, 39, 44), Color.FromArgb(48, 52, 59), Color.FromArgb(232, 234, 237)),
            "Paper" => (Color.FromArgb(255, 248, 230), Color.FromArgb(235, 218, 183), Color.FromArgb(62, 51, 39)),
            _ => (Color.White, Color.FromArgb(244, 245, 247), Color.FromArgb(32, 35, 40))
        };
        BackColor = surface; _text.BackColor = surface; _text.ForeColor = ink;
        _header.BackColor = header; _title.BackColor = header; _title.ForeColor = ink;
        foreach (Button button in new[] { _new, _menu, _pin, _close })
        { button.BackColor = header; button.ForeColor = ink; }
        _menuItems.BackColor = header; _menuItems.ForeColor = ink;
    }

    protected override void OnClosing(CancelEventArgs e)
    {
        e.Cancel = !_context.CloseNote(this);
        base.OnClosing(e);
    }

    protected override void WndProc(ref Message message)
    {
        if (message.Msg == WmNcHitTest)
        {
            Point point = PointToClient(new Point(unchecked((short)(long)message.LParam),
                unchecked((short)((long)message.LParam >> 16))));
            int x = point.X < 6 ? -1 : point.X >= Width - 6 ? 1 : 0;
            int y = point.Y < 6 ? -1 : point.Y >= Height - 6 ? 1 : 0;
            int hit = (x, y) switch
            {
                (-1, -1) => 13, (1, -1) => 14, (-1, 1) => 16, (1, 1) => 17,
                (-1, 0) => 10, (1, 0) => 11, (0, -1) => 12, (0, 1) => 15, _ => 1
            };
            if (hit != 1) { message.Result = (IntPtr)hit; return; }
        }
        base.WndProc(ref message);
    }
}
