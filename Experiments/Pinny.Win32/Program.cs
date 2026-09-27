using System.Runtime.InteropServices;
using System.Text;
using Pinny;

namespace Pinny.Win32Variant;

internal static class Program
{
    private const string WindowClass = "PinnyNativeNote";
    private const int EditId = 100, NewId = 1, PinId = 2, DeleteId = 3, QuitId = 4,
        LightId = 5, DarkId = 6, PaperId = 7;
    private const uint WmSetFont = 0x30, EmSetMargins = 0xD3, EmGetLineCount = 0xBA;
    private const int HeaderHeight = 32, ResizeBorder = 6;
    private static readonly Native.WindowProc Callback = HandleMessage;
    private static readonly Dictionary<IntPtr, NoteWindow> Windows = [];
    private static readonly Dictionary<string, Palette> Palettes = [];
    private static IntPtr _font;
    private static bool _quitting;
    private static bool _saveErrorShown;

    private sealed class NoteWindow(NoteState state)
    {
        public Guid Id = state.Id;
        public IntPtr Edit;
        public IntPtr Menu;
        public bool Pinned = state.IsPinned;
        public string Theme = state.Theme is "Dark" or "Paper" ? state.Theme : "Light";
        public int HoveredButton;
        public int PressedButton;
        public bool ScrollbarShown;
    }

    private sealed class Palette(uint surface, uint header, uint border, uint hover, uint ink, uint muted)
    {
        public readonly IntPtr Surface = Native.CreateSolidBrush(surface);
        public readonly IntPtr Header = Native.CreateSolidBrush(header);
        public readonly IntPtr Border = Native.CreateSolidBrush(border);
        public readonly IntPtr Hover = Native.CreateSolidBrush(hover);
        public readonly uint SurfaceColor = surface, InkColor = ink, MutedColor = muted;

        public void Dispose()
        {
            Native.DeleteObject(Surface);
            Native.DeleteObject(Header);
            Native.DeleteObject(Border);
            Native.DeleteObject(Hover);
        }
    }

    private static uint Rgb(byte red, byte green, byte blue) =>
        (uint)(red | (green << 8) | (blue << 16));

    [STAThread]
    private static void Main(string[] args)
    {
        NoteStorage.SetDataDirectory(args is ["--data-dir", var directory]
            ? directory : Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Pinny.Baseline", "Win32"));
        try
        {
            Run();
        }
        catch (Exception ex)
        {
            Native.MessageBoxW(IntPtr.Zero, $"Pinny could not start. Saved data was not changed.\n\n{ex.Message}", "Pinny", 0x10);
        }
        finally
        {
            foreach (Palette palette in Palettes.Values) palette.Dispose();
            if (_font != IntPtr.Zero) Native.DeleteObject(_font);
        }
    }

    private static void Run()
    {
        var windowClass = new Native.WNDCLASSEX
        {
            cbSize = (uint)Marshal.SizeOf<Native.WNDCLASSEX>(),
            lpfnWndProc = Marshal.GetFunctionPointerForDelegate(Callback),
            hCursor = Native.LoadCursorW(IntPtr.Zero, (IntPtr)32512),
            lpszClassName = WindowClass
        };
        if (Native.RegisterClassExW(ref windowClass) == 0)
            throw new InvalidOperationException($"Could not register window class: {Marshal.GetLastWin32Error()}");

        Palettes["Light"] = new Palette(Rgb(255, 255, 255), Rgb(244, 245, 247),
            Rgb(200, 205, 211), Rgb(225, 229, 235), Rgb(32, 35, 40), Rgb(85, 91, 99));
        Palettes["Dark"] = new Palette(Rgb(36, 39, 44), Rgb(48, 52, 59),
            Rgb(76, 84, 94), Rgb(67, 73, 82), Rgb(232, 234, 237), Rgb(189, 197, 205));
        Palettes["Paper"] = new Palette(Rgb(255, 248, 230), Rgb(235, 218, 183),
            Rgb(198, 177, 133), Rgb(222, 201, 161), Rgb(62, 51, 39), Rgb(107, 88, 59));
        _font = Native.CreateFontW(-16, 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, "Segoe UI");

        List<NoteState> notes = NoteStorage.LoadAll();
        if (notes.Count == 0) notes.Add(new NoteState());
        foreach (NoteState note in notes) ShowNote(note);

        while (Native.GetMessageW(out Native.MSG message, IntPtr.Zero, 0, 0) > 0)
        {
            Native.TranslateMessage(ref message);
            Native.DispatchMessageW(ref message);
        }
    }

    private static void ShowNote(NoteState state)
    {
        int width = (int)Math.Clamp(double.IsFinite(state.Width) ? state.Width : 320, 210, 10000);
        int height = (int)Math.Clamp(double.IsFinite(state.Height) ? state.Height : 320, 120, 10000);
        int sx = Native.GetSystemMetrics(76), sy = Native.GetSystemMetrics(77);
        int sw = Native.GetSystemMetrics(78), sh = Native.GetSystemMetrics(79);
        int x = double.IsFinite(state.Left) ? (int)state.Left : sx + (sw - width) / 2;
        int y = double.IsFinite(state.Top) ? (int)state.Top : sy + (sh - height) / 2;
        x = Math.Clamp(x, sx - width + 48, sx + sw - 48);
        y = Math.Clamp(y, sy, sy + sh - 48);

        int style = Native.WS_POPUP | Native.WS_THICKFRAME | Native.WS_SYSMENU | Native.WS_MINIMIZEBOX;
        IntPtr handle = Native.CreateWindowExW(Native.WS_EX_APPWINDOW, WindowClass, "Pinny", style,
            x, y, width, height, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero, IntPtr.Zero);
        if (handle == IntPtr.Zero)
            throw new InvalidOperationException($"Could not create window: {Marshal.GetLastWin32Error()}");
        var note = new NoteWindow(state);
        Windows.Add(handle, note);

        note.Menu = Native.CreatePopupMenu();
        Native.AppendMenuW(note.Menu, Native.MF_STRING, (IntPtr)LightId, "Light");
        Native.AppendMenuW(note.Menu, Native.MF_STRING, (IntPtr)DarkId, "Dark");
        Native.AppendMenuW(note.Menu, Native.MF_STRING, (IntPtr)PaperId, "Paper");
        Native.AppendMenuW(note.Menu, Native.MF_SEPARATOR, IntPtr.Zero, null);
        Native.AppendMenuW(note.Menu, Native.MF_STRING, (IntPtr)QuitId, "Quit Pinny");

        note.Edit = Native.CreateWindowExW(0, "EDIT", state.Text ?? "",
            Native.WS_CHILD | Native.WS_VISIBLE | Native.WS_VSCROLL | Native.ES_MULTILINE |
            Native.ES_AUTOVSCROLL | Native.ES_WANTRETURN,
            1, HeaderHeight, width - 2, height - HeaderHeight - 1,
            handle, (IntPtr)EditId, IntPtr.Zero, IntPtr.Zero);
        if (note.Edit == IntPtr.Zero)
            throw new InvalidOperationException($"Could not create editor: {Marshal.GetLastWin32Error()}");
        Native.SendMessageW(note.Edit, 0x00C5, (IntPtr)int.MaxValue, IntPtr.Zero); // EM_LIMITTEXT
        Native.SendMessageW(note.Edit, EmSetMargins, (IntPtr)3, (IntPtr)(10 | (10 << 16)));
        Native.SendMessageW(note.Edit, WmSetFont, _font, (IntPtr)1);
        Native.GetClientRect(handle, out Native.RECT client);
        Native.MoveWindow(note.Edit, 1, HeaderHeight + 8, client.right - 2,
            client.bottom - HeaderHeight - 16, true);
        Native.ShowScrollBar(note.Edit, 1, false);
        UpdateScrollbar(note);
        Native.ShowWindow(handle, Native.SW_SHOW);
        if (note.Pinned)
            Native.SetWindowPos(handle, (IntPtr)Native.HWND_TOPMOST, 0, 0, 0, 0,
                Native.SWP_NOMOVE | Native.SWP_NOSIZE | Native.SWP_NOACTIVATE);
        Native.UpdateWindow(handle);
        Native.SetFocus(note.Edit);
    }

    private static IntPtr HandleMessage(IntPtr hwnd, uint message, IntPtr wParam, IntPtr lParam)
    {
        if (message == Native.WM_NCCALCSIZE)
            return IntPtr.Zero;

        if (!Windows.TryGetValue(hwnd, out NoteWindow? note))
            return Native.DefWindowProcW(hwnd, message, wParam, lParam);

        try
        {
            switch (message)
            {
                case Native.WM_NCHITTEST:
                    Native.POINT location = new()
                    {
                        x = unchecked((short)(long)lParam),
                        y = unchecked((short)((long)lParam >> 16))
                    };
                    Native.ScreenToClient(hwnd, ref location);
                    Native.GetClientRect(hwnd, out Native.RECT size);
                    int edgeX = location.x < ResizeBorder ? -1 : location.x >= size.right - ResizeBorder ? 1 : 0;
                    int edgeY = location.y < ResizeBorder ? -1 : location.y >= size.bottom - ResizeBorder ? 1 : 0;
                    int hit = (edgeX, edgeY) switch
                    {
                        (-1, -1) => 13, (1, -1) => 14, (-1, 1) => 16, (1, 1) => 17,
                        (-1, 0) => 10, (1, 0) => 11, (0, -1) => 12, (0, 1) => 15,
                        _ => 1
                    };
                    return (IntPtr)hit;
                case Native.WM_ERASEBKGND:
                    return (IntPtr)1;
                case Native.WM_PAINT:
                    PaintHeader(hwnd, note);
                    return IntPtr.Zero;
                case Native.WM_MOUSEMOVE:
                    int hover = ButtonAt(hwnd, lParam);
                    if (hover != note.HoveredButton)
                    {
                        note.HoveredButton = hover;
                        Native.InvalidateRect(hwnd, IntPtr.Zero, false);
                    }
                    var tracking = new Native.TRACKMOUSEEVENT
                    {
                        cbSize = (uint)Marshal.SizeOf<Native.TRACKMOUSEEVENT>(),
                        dwFlags = 2,
                        hwndTrack = hwnd
                    };
                    Native.TrackMouseEvent(ref tracking);
                    return IntPtr.Zero;
                case Native.WM_MOUSELEAVE:
                    note.HoveredButton = 0;
                    Native.InvalidateRect(hwnd, IntPtr.Zero, false);
                    return IntPtr.Zero;
                case Native.WM_LBUTTONDOWN:
                    note.PressedButton = ButtonAt(hwnd, lParam);
                    if (note.PressedButton != 0)
                        Native.SetCapture(hwnd);
                    else if (unchecked((short)((long)lParam >> 16)) < HeaderHeight)
                    {
                        Native.ReleaseCapture();
                        Native.SendMessageW(hwnd, Native.WM_NCLBUTTONDOWN, (IntPtr)2, IntPtr.Zero);
                    }
                    return IntPtr.Zero;
                case Native.WM_LBUTTONUP:
                    Native.ReleaseCapture();
                    int pressed = note.PressedButton;
                    note.PressedButton = 0;
                    if (pressed != 0 && pressed == ButtonAt(hwnd, lParam))
                        ClickButton(hwnd, note, pressed);
                    return IntPtr.Zero;
                case Native.WM_SIZE:
                    Native.GetClientRect(hwnd, out Native.RECT client);
                    Native.MoveWindow(note.Edit, 1, HeaderHeight + 8,
                        Math.Max(1, client.right - 2), Math.Max(1, client.bottom - HeaderHeight - 16), true);
                    UpdateScrollbar(note);
                    QueueSave(hwnd);
                    return IntPtr.Zero;
                case Native.WM_MOVE:
                    QueueSave(hwnd);
                    return IntPtr.Zero;
                case Native.WM_SETFOCUS:
                    Native.SetFocus(note.Edit);
                    return IntPtr.Zero;
                case Native.WM_COMMAND:
                    int id = (int)((long)wParam & 0xffff);
                    int notification = (int)(((long)wParam >> 16) & 0xffff);
                    if (id == EditId && notification == 0x300)
                    {
                        UpdateScrollbar(note);
                        QueueSave(hwnd);
                    }
                    else if (id == NewId) NewNote(hwnd);
                    else if (id == PinId) TogglePin(hwnd, note);
                    else if (id == DeleteId) Native.SendMessageW(hwnd, Native.WM_CLOSE, IntPtr.Zero, IntPtr.Zero);
                    else if (id == QuitId) Quit();
                    else if (id is LightId or DarkId or PaperId)
                    {
                        note.Theme = id == DarkId ? "Dark" : id == PaperId ? "Paper" : "Light";
                        Native.InvalidateRect(note.Edit, IntPtr.Zero, true);
                        Native.InvalidateRect(hwnd, IntPtr.Zero, false);
                        QueueSave(hwnd);
                    }
                    return IntPtr.Zero;
                case Native.WM_CTLCOLOREDIT:
                    Palette palette = Palettes[note.Theme];
                    Native.SetTextColor(wParam, palette.InkColor);
                    Native.SetBkColor(wParam, palette.SurfaceColor);
                    return palette.Surface;
                case Native.WM_TIMER:
                    Native.KillTimer(hwnd, (IntPtr)1);
                    TrySave();
                    return IntPtr.Zero;
                case Native.WM_CLOSE:
                    if (_quitting || TrySave(hwnd)) Native.DestroyWindow(hwnd);
                    return IntPtr.Zero;
                case Native.WM_DESTROY:
                    Native.KillTimer(hwnd, (IntPtr)1);
                    Native.DestroyMenu(note.Menu);
                    Windows.Remove(hwnd);
                    if (!_quitting && !TrySave())
                        Native.MessageBoxW(IntPtr.Zero, "A deleted note could not be saved. Check the data file.", "Pinny", 0x30);
                    if (Windows.Count == 0) Native.PostQuitMessage(0);
                    return IntPtr.Zero;
            }
        }
        catch (Exception ex)
        {
            Native.MessageBoxW(hwnd, ex.Message, "Pinny error", 0x10);
        }
        return Native.DefWindowProcW(hwnd, message, wParam, lParam);
    }

    private static void QueueSave(IntPtr hwnd)
    {
        Native.KillTimer(hwnd, (IntPtr)1);
        Native.SetTimer(hwnd, (IntPtr)1, 500, IntPtr.Zero);
    }

    private static int ButtonAt(IntPtr hwnd, IntPtr lParam)
    {
        int x = unchecked((short)(long)lParam);
        int y = unchecked((short)((long)lParam >> 16));
        if (y < 0 || y >= HeaderHeight) return 0;
        Native.GetClientRect(hwnd, out Native.RECT client);
        return x >= client.right - 36 ? 4 :
            x >= client.right - 72 ? 3 :
            x >= client.right - 117 ? 2 :
            x >= client.right - 151 ? 1 : 0;
    }

    private static void ClickButton(IntPtr hwnd, NoteWindow note, int button)
    {
        switch (button)
        {
            case 1:
                NewNote(hwnd);
                break;
            case 2:
                Native.GetClientRect(hwnd, out Native.RECT client);
                Native.POINT point = new() { x = client.right - 117, y = HeaderHeight };
                Native.ClientToScreen(hwnd, ref point);
                Native.SetForegroundWindow(hwnd);
                uint choice = Native.TrackPopupMenu(note.Menu, Native.TPM_RETURNCMD,
                    point.x, point.y, 0, hwnd, IntPtr.Zero);
                if (choice != 0)
                    Native.SendMessageW(hwnd, Native.WM_COMMAND, (IntPtr)choice, IntPtr.Zero);
                break;
            case 3:
                TogglePin(hwnd, note);
                break;
            case 4:
                Native.SendMessageW(hwnd, Native.WM_CLOSE, IntPtr.Zero, IntPtr.Zero);
                break;
        }
    }

    private static void PaintHeader(IntPtr hwnd, NoteWindow note)
    {
        IntPtr dc = Native.BeginPaint(hwnd, out Native.PAINTSTRUCT paint);
        if (dc == IntPtr.Zero) return;
        try
        {
            Native.GetClientRect(hwnd, out Native.RECT bounds);
            Palette palette = Palettes[note.Theme];
            Native.FillRect(dc, ref bounds, palette.Surface);
            Native.RECT header = new() { left = 0, top = 0, right = bounds.right, bottom = HeaderHeight };
            Native.FillRect(dc, ref header, palette.Header);
            Native.RECT divider = new() { left = 0, top = HeaderHeight - 1, right = bounds.right, bottom = HeaderHeight };
            Native.FillRect(dc, ref divider, palette.Border);
            Native.FrameRect(dc, ref bounds, palette.Border);
            Native.SetBkMode(dc, 1); // TRANSPARENT
            IntPtr previousFont = Native.SelectObject(dc, _font);
            try
            {
                Native.SetTextColor(dc, palette.MutedColor);
                Native.RECT title = new() { left = 10, top = 1, right = Math.Max(10, bounds.right - 151), bottom = HeaderHeight - 1 };
                Native.DrawTextW(dc, "Pinny", 5, ref title, Native.DT_VCENTER | Native.DT_SINGLELINE);

                string[] labels = ["+", "Menu", note.Pinned ? "◆" : "◇", "×"];
                int[] widths = [34, 45, 36, 36];
                int left = bounds.right - 151;
                for (int i = 0; i < labels.Length; i++)
                {
                    Native.RECT button = new() { left = left, top = 1, right = left + widths[i], bottom = HeaderHeight - 1 };
                    if (note.HoveredButton == i + 1)
                        Native.FillRect(dc, ref button, palette.Hover);
                    Native.SetTextColor(dc, palette.InkColor);
                    Native.DrawTextW(dc, labels[i], labels[i].Length, ref button,
                        Native.DT_CENTER | Native.DT_VCENTER | Native.DT_SINGLELINE);
                    left += widths[i];
                }
            }
            finally { Native.SelectObject(dc, previousFont); }
        }
        finally { Native.EndPaint(hwnd, ref paint); }
    }

    private static void NewNote(IntPtr source)
    {
        NoteState current = Capture(source, Windows[source]);
        ShowNote(new NoteState
        {
            Left = current.Left + 32, Top = current.Top + 32,
            Width = current.Width, Height = current.Height,
            IsPinned = current.IsPinned, Theme = current.Theme
        });
        TrySave();
    }

    private static void TogglePin(IntPtr hwnd, NoteWindow note)
    {
        note.Pinned = !note.Pinned;
        Native.SetWindowPos(hwnd, (IntPtr)(note.Pinned ? Native.HWND_TOPMOST : Native.HWND_NOTOPMOST),
            0, 0, 0, 0, Native.SWP_NOMOVE | Native.SWP_NOSIZE | Native.SWP_NOACTIVATE);
        Native.InvalidateRect(hwnd, IntPtr.Zero, false);
        QueueSave(hwnd);
    }

    private static void UpdateScrollbar(NoteWindow note)
    {
        if (note.Edit == IntPtr.Zero) return;
        Native.GetClientRect(note.Edit, out Native.RECT area);
        long lines = Native.SendMessageW(note.Edit, EmGetLineCount, IntPtr.Zero, IntPtr.Zero).ToInt64();
        bool show = lines * 16 > area.bottom;
        if (show == note.ScrollbarShown) return;
        Native.ShowScrollBar(note.Edit, 1, show);
        note.ScrollbarShown = show;
    }

    private static NoteState Capture(IntPtr hwnd, NoteWindow note)
    {
        Native.GetWindowRect(hwnd, out Native.RECT rect);
        int length = Native.GetWindowTextLengthW(note.Edit);
        var buffer = new StringBuilder(length + 1);
        Native.GetWindowTextW(note.Edit, buffer, buffer.Capacity);
        return new NoteState
        {
            Id = note.Id, Text = buffer.ToString(), Left = rect.left, Top = rect.top,
            Width = rect.right - rect.left, Height = rect.bottom - rect.top,
            IsPinned = note.Pinned, Theme = note.Theme
        };
    }

    private static bool TrySave(IntPtr excluded = default)
    {
        try
        {
            NoteStorage.SaveAll(Windows.Where(pair => pair.Key != excluded)
                .Select(pair => Capture(pair.Key, pair.Value)));
            _saveErrorShown = false;
            return true;
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException or System.Text.Json.JsonException)
        {
            if (!_saveErrorShown)
            {
                _saveErrorShown = true;
                Native.MessageBoxW(IntPtr.Zero, $"Could not save notes.\n\n{ex.Message}", "Pinny", 0x30);
            }
            return false;
        }
    }

    private static void Quit()
    {
        if (!TrySave()) return;
        _quitting = true;
        foreach (IntPtr handle in Windows.Keys.ToArray()) Native.DestroyWindow(handle);
    }
}
