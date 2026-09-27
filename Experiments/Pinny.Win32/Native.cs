using System.Runtime.InteropServices;
using System.Text;

namespace Pinny.Win32Variant;

internal static class Native
{
    internal const int WM_SIZE = 0x0005, WM_MOVE = 0x0003, WM_CLOSE = 0x0010,
        WM_DESTROY = 0x0002, WM_COMMAND = 0x0111, WM_TIMER = 0x0113,
        WM_CTLCOLOREDIT = 0x0133, WM_ERASEBKGND = 0x0014, WM_SETFOCUS = 0x0007,
        WM_NCCALCSIZE = 0x0083, WM_NCHITTEST = 0x0084, WM_PAINT = 0x000F,
        WM_MOUSEMOVE = 0x0200, WM_MOUSELEAVE = 0x02A3, WM_LBUTTONDOWN = 0x0201,
        WM_LBUTTONUP = 0x0202, WM_NCLBUTTONDOWN = 0x00A1;
    internal const int WS_OVERLAPPEDWINDOW = 0x00CF0000, WS_VISIBLE = 0x10000000,
        WS_CHILD = 0x40000000, WS_VSCROLL = 0x00200000, WS_EX_CLIENTEDGE = 0x200,
        WS_POPUP = unchecked((int)0x80000000), WS_THICKFRAME = 0x00040000,
        WS_SYSMENU = 0x00080000, WS_MINIMIZEBOX = 0x00020000, WS_EX_APPWINDOW = 0x00040000;
    internal const int ES_MULTILINE = 0x0004, ES_AUTOVSCROLL = 0x0040, ES_WANTRETURN = 0x1000;
    internal const int SW_SHOW = 5, HWND_TOPMOST = -1, HWND_NOTOPMOST = -2,
        SWP_NOMOVE = 2, SWP_NOSIZE = 1, SWP_NOACTIVATE = 16;
    internal const uint MF_STRING = 0, MF_POPUP = 0x10, MF_SEPARATOR = 0x800;
    internal const uint TPM_RETURNCMD = 0x0100, DT_CENTER = 1, DT_VCENTER = 4, DT_SINGLELINE = 0x20;

    [StructLayout(LayoutKind.Sequential)]
    internal struct WNDCLASSEX
    {
        public uint cbSize, style;
        public IntPtr lpfnWndProc;
        public int cbClsExtra, cbWndExtra;
        public IntPtr hInstance, hIcon, hCursor, hbrBackground;
        [MarshalAs(UnmanagedType.LPWStr)] public string? lpszMenuName;
        [MarshalAs(UnmanagedType.LPWStr)] public string lpszClassName;
        public IntPtr hIconSm;
    }

    [StructLayout(LayoutKind.Sequential)]
    internal struct POINT { public int x, y; }
    [StructLayout(LayoutKind.Sequential)]
    internal struct MSG
    {
        public IntPtr hwnd;
        public uint message;
        public IntPtr wParam, lParam;
        public uint time;
        public POINT pt;
        public uint lPrivate;
    }
    [StructLayout(LayoutKind.Sequential)]
    internal struct RECT { public int left, top, right, bottom; }
    [StructLayout(LayoutKind.Sequential)]
    internal struct PAINTSTRUCT
    {
        public IntPtr hdc;
        public bool fErase;
        public RECT rcPaint;
        public bool fRestore, fIncUpdate;
        [MarshalAs(UnmanagedType.ByValArray, SizeConst = 32)] public byte[] rgbReserved;
    }
    [StructLayout(LayoutKind.Sequential)]
    internal struct TRACKMOUSEEVENT
    {
        public uint cbSize, dwFlags;
        public IntPtr hwndTrack;
        public uint dwHoverTime;
    }

    [UnmanagedFunctionPointer(CallingConvention.Winapi)]
    internal delegate IntPtr WindowProc(IntPtr hwnd, uint message, IntPtr wParam, IntPtr lParam);

    [DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    internal static extern ushort RegisterClassExW(ref WNDCLASSEX windowClass);
    [DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    internal static extern IntPtr CreateWindowExW(int extendedStyle, string className, string windowName,
        int style, int x, int y, int width, int height, IntPtr parent, IntPtr menu, IntPtr instance, IntPtr parameter);
    [DllImport("user32.dll")] internal static extern IntPtr DefWindowProcW(IntPtr hwnd, uint message, IntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll")] internal static extern bool DestroyWindow(IntPtr hwnd);
    [DllImport("user32.dll")] internal static extern bool ShowWindow(IntPtr hwnd, int command);
    [DllImport("user32.dll")] internal static extern bool UpdateWindow(IntPtr hwnd);
    [DllImport("user32.dll")] internal static extern int GetMessageW(out MSG message, IntPtr hwnd, uint min, uint max);
    [DllImport("user32.dll")] internal static extern bool TranslateMessage(ref MSG message);
    [DllImport("user32.dll")] internal static extern IntPtr DispatchMessageW(ref MSG message);
    [DllImport("user32.dll")] internal static extern void PostQuitMessage(int code);
    [DllImport("user32.dll")] internal static extern bool GetWindowRect(IntPtr hwnd, out RECT rect);
    [DllImport("user32.dll")] internal static extern bool GetClientRect(IntPtr hwnd, out RECT rect);
    [DllImport("user32.dll")] internal static extern bool ClientToScreen(IntPtr hwnd, ref POINT point);
    [DllImport("user32.dll")] internal static extern bool ScreenToClient(IntPtr hwnd, ref POINT point);
    [DllImport("user32.dll")] internal static extern bool MoveWindow(IntPtr hwnd, int x, int y, int width, int height, bool repaint);
    [DllImport("user32.dll")] internal static extern bool SetWindowPos(IntPtr hwnd, IntPtr after, int x, int y, int width, int height, uint flags);
    [DllImport("user32.dll")] internal static extern IntPtr SetFocus(IntPtr hwnd);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] internal static extern int GetWindowTextW(IntPtr hwnd, StringBuilder text, int length);
    [DllImport("user32.dll")] internal static extern int GetWindowTextLengthW(IntPtr hwnd);
    [DllImport("user32.dll")] internal static extern IntPtr SendMessageW(IntPtr hwnd, uint message, IntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll")] internal static extern IntPtr SetTimer(IntPtr hwnd, IntPtr id, uint milliseconds, IntPtr callback);
    [DllImport("user32.dll")] internal static extern bool KillTimer(IntPtr hwnd, IntPtr id);
    [DllImport("user32.dll")] internal static extern IntPtr CreateMenu();
    [DllImport("user32.dll")] internal static extern IntPtr CreatePopupMenu();
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] internal static extern bool AppendMenuW(IntPtr menu, uint flags, IntPtr item, string? text);
    [DllImport("user32.dll")] internal static extern bool SetMenu(IntPtr hwnd, IntPtr menu);
    [DllImport("user32.dll")] internal static extern bool DestroyMenu(IntPtr menu);
    [DllImport("user32.dll")] internal static extern uint TrackPopupMenu(IntPtr menu, uint flags, int x, int y, int reserved, IntPtr hwnd, IntPtr rect);
    [DllImport("user32.dll")] internal static extern bool TrackMouseEvent(ref TRACKMOUSEEVENT tracking);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] internal static extern int MessageBoxW(IntPtr hwnd, string text, string caption, uint type);
    [DllImport("user32.dll")] internal static extern int GetSystemMetrics(int index);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] internal static extern IntPtr LoadCursorW(IntPtr instance, IntPtr cursor);
    [DllImport("gdi32.dll")] internal static extern IntPtr CreateSolidBrush(uint color);
    [DllImport("gdi32.dll")] internal static extern IntPtr GetStockObject(int index);
    [DllImport("gdi32.dll", CharSet = CharSet.Unicode)] internal static extern IntPtr CreateFontW(
        int height, int width, int escapement, int orientation, int weight, uint italic,
        uint underline, uint strikeOut, uint charSet, uint outputPrecision,
        uint clipPrecision, uint quality, uint pitchAndFamily, string faceName);
    [DllImport("gdi32.dll")] internal static extern bool DeleteObject(IntPtr obj);
    [DllImport("gdi32.dll")] internal static extern IntPtr SelectObject(IntPtr dc, IntPtr obj);
    [DllImport("gdi32.dll")] internal static extern int SetBkMode(IntPtr dc, int mode);
    [DllImport("gdi32.dll")] internal static extern uint SetTextColor(IntPtr dc, uint color);
    [DllImport("gdi32.dll")] internal static extern uint SetBkColor(IntPtr dc, uint color);
    [DllImport("user32.dll")] internal static extern bool InvalidateRect(IntPtr hwnd, IntPtr rect, bool erase);
    [DllImport("user32.dll")] internal static extern IntPtr BeginPaint(IntPtr hwnd, out PAINTSTRUCT paint);
    [DllImport("user32.dll")] internal static extern bool EndPaint(IntPtr hwnd, ref PAINTSTRUCT paint);
    [DllImport("user32.dll")] internal static extern int FillRect(IntPtr dc, ref RECT rect, IntPtr brush);
    [DllImport("user32.dll")] internal static extern int FrameRect(IntPtr dc, ref RECT rect, IntPtr brush);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] internal static extern int DrawTextW(IntPtr dc, string text, int length, ref RECT rect, uint format);
    [DllImport("user32.dll")] internal static extern bool ReleaseCapture();
    [DllImport("user32.dll")] internal static extern IntPtr SetCapture(IntPtr hwnd);
    [DllImport("user32.dll")] internal static extern bool SetForegroundWindow(IntPtr hwnd);
    [DllImport("user32.dll")] internal static extern bool ShowScrollBar(IntPtr hwnd, int bar, bool show);
}
