#![cfg(windows)]
#![windows_subsystem = "windows"]

// The Win32 ABI declarations below are direct system calls. Basic types and
// system metrics use Microsoft's thin windows-sys bindings.
use std::ffi::c_void;
use std::mem::{size_of, zeroed};
use std::ptr::null;
use std::sync::{Mutex, OnceLock};
use windows_sys::Win32::Foundation::RECT;
use windows_sys::Win32::UI::WindowsAndMessaging::GetSystemMetrics;

type Handle = isize;
const CLASS: &str = "PinnyNativeNoteRust";
const HEADER: i32 = 32;
const EDGE: i32 = 6;
const EDIT_ID: usize = 100;
const NEW_ID: usize = 1;
const PIN_ID: usize = 2;
const DELETE_ID: usize = 3;
const QUIT_ID: usize = 4;
const LIGHT_ID: usize = 5;
const DARK_ID: usize = 6;
const PAPER_ID: usize = 7;

const WM_DESTROY: u32 = 0x0002;
const WM_MOVE: u32 = 0x0003;
const WM_SIZE: u32 = 0x0005;
const WM_SETFOCUS: u32 = 0x0007;
const WM_PAINT: u32 = 0x000f;
const WM_CLOSE: u32 = 0x0010;
const WM_ERASEBKGND: u32 = 0x0014;
const WM_SETFONT: u32 = 0x0030;
const WM_NCCALCSIZE: u32 = 0x0083;
const WM_NCHITTEST: u32 = 0x0084;
const WM_NCLBUTTONDOWN: u32 = 0x00a1;
const WM_NCDESTROY: u32 = 0x0082;
const WM_COMMAND: u32 = 0x0111;
const WM_CTLCOLOREDIT: u32 = 0x0133;
const WM_MOUSEMOVE: u32 = 0x0200;
const WM_LBUTTONDOWN: u32 = 0x0201;
const WM_LBUTTONUP: u32 = 0x0202;
const WM_MOUSELEAVE: u32 = 0x02a3;
const EM_LIMITTEXT: u32 = 0x00c5;
const EM_GETLINECOUNT: u32 = 0x00ba;
const EM_SETMARGINS: u32 = 0x00d3;
const WS_POPUP: u32 = 0x80000000;
const WS_THICKFRAME: u32 = 0x00040000;
const WS_SYSMENU: u32 = 0x00080000;
const WS_MINIMIZEBOX: u32 = 0x00020000;
const WS_CHILD: u32 = 0x40000000;
const WS_VISIBLE: u32 = 0x10000000;
const WS_VSCROLL: u32 = 0x00200000;
const WS_EX_APPWINDOW: u32 = 0x00040000;
const ES_MULTILINE: u32 = 0x0004;
const ES_AUTOVSCROLL: u32 = 0x0040;
const ES_WANTRETURN: u32 = 0x1000;
const MF_SEPARATOR: u32 = 0x0800;
const TPM_RETURNCMD: u32 = 0x0100;
const DT_CENTER: u32 = 1;
const DT_VCENTER: u32 = 4;
const DT_SINGLELINE: u32 = 0x20;
const SWP_NOSIZE: u32 = 1;
const SWP_NOMOVE: u32 = 2;
const SWP_NOACTIVATE: u32 = 16;
const GWLP_USERDATA: i32 = -21;

#[repr(C)]
struct WndClassEx {
    size: u32, style: u32,
    proc: Option<unsafe extern "system" fn(Handle, u32, usize, isize) -> isize>,
    cls_extra: i32, wnd_extra: i32,
    instance: Handle, icon: Handle, cursor: Handle, background: Handle,
    menu_name: *const u16, class_name: *const u16, small_icon: Handle,
}
#[repr(C)]
#[derive(Clone, Copy)]
struct Point { x: i32, y: i32 }
#[repr(C)]
struct Msg {
    hwnd: Handle, message: u32, wparam: usize, lparam: isize,
    time: u32, pt: Point, private: u32,
}
#[repr(C)]
struct PaintStruct {
    dc: Handle, erase: i32, paint: RECT, restore: i32, inc_update: i32,
    reserved: [u8; 32],
}
#[repr(C)]
struct MouseTrack {
    size: u32, flags: u32, hwnd: Handle, hover_time: u32,
}
struct Palette {
    surface: Handle, header: Handle, border: Handle, hover: Handle,
    surface_color: u32, ink: u32, muted: u32,
}
struct Note {
    edit: Handle, menu: Handle, theme: usize, pinned: bool,
    hovered: usize, pressed: usize, scrollbar: bool,
}
struct InitialNote {
    x: i32, y: i32, width: i32, height: i32,
    text: String, theme: usize, pinned: bool,
}

static WINDOWS: Mutex<Vec<Handle>> = Mutex::new(Vec::new());
static PALETTES: OnceLock<[Palette; 3]> = OnceLock::new();
static FONT: OnceLock<Handle> = OnceLock::new();

#[link(name = "user32")]
extern "system" {
    fn RegisterClassExW(class: *const WndClassEx) -> u16;
    fn CreateWindowExW(ex: u32, class: *const u16, name: *const u16, style: u32,
        x: i32, y: i32, width: i32, height: i32, parent: Handle, menu: Handle,
        instance: Handle, param: *const c_void) -> Handle;
    fn DefWindowProcW(hwnd: Handle, msg: u32, wp: usize, lp: isize) -> isize;
    fn DestroyWindow(hwnd: Handle) -> i32;
    fn ShowWindow(hwnd: Handle, command: i32) -> i32;
    fn UpdateWindow(hwnd: Handle) -> i32;
    fn GetMessageW(message: *mut Msg, hwnd: Handle, min: u32, max: u32) -> i32;
    fn TranslateMessage(message: *const Msg) -> i32;
    fn DispatchMessageW(message: *const Msg) -> isize;
    fn PostQuitMessage(code: i32);
    fn GetClientRect(hwnd: Handle, rect: *mut RECT) -> i32;
    fn GetWindowRect(hwnd: Handle, rect: *mut RECT) -> i32;
    fn ScreenToClient(hwnd: Handle, point: *mut Point) -> i32;
    fn ClientToScreen(hwnd: Handle, point: *mut Point) -> i32;
    fn MoveWindow(hwnd: Handle, x: i32, y: i32, w: i32, h: i32, repaint: i32) -> i32;
    fn SetWindowPos(hwnd: Handle, after: Handle, x: i32, y: i32, w: i32, h: i32, flags: u32) -> i32;
    fn SetFocus(hwnd: Handle) -> Handle;
    fn SendMessageW(hwnd: Handle, msg: u32, wp: usize, lp: isize) -> isize;
    fn InvalidateRect(hwnd: Handle, rect: *const RECT, erase: i32) -> i32;
    fn BeginPaint(hwnd: Handle, paint: *mut PaintStruct) -> Handle;
    fn EndPaint(hwnd: Handle, paint: *const PaintStruct) -> i32;
    fn FillRect(dc: Handle, rect: *const RECT, brush: Handle) -> i32;
    fn FrameRect(dc: Handle, rect: *const RECT, brush: Handle) -> i32;
    fn DrawTextW(dc: Handle, text: *const u16, len: i32, rect: *mut RECT, format: u32) -> i32;
    fn CreatePopupMenu() -> Handle;
    fn AppendMenuW(menu: Handle, flags: u32, id: usize, text: *const u16) -> i32;
    fn DestroyMenu(menu: Handle) -> i32;
    fn TrackPopupMenu(menu: Handle, flags: u32, x: i32, y: i32, reserved: i32,
        hwnd: Handle, rect: *const RECT) -> u32;
    fn TrackMouseEvent(event: *mut MouseTrack) -> i32;
    fn SetForegroundWindow(hwnd: Handle) -> i32;
    fn SetCapture(hwnd: Handle) -> Handle;
    fn ReleaseCapture() -> i32;
    fn ShowScrollBar(hwnd: Handle, bar: i32, show: i32) -> i32;
    fn LoadCursorW(instance: Handle, cursor: *const u16) -> Handle;
    fn LoadImageW(instance: Handle, name: *const u16, image_type: u32,
        width: i32, height: i32, flags: u32) -> Handle;
    fn DestroyIcon(icon: Handle) -> i32;
    fn MessageBoxW(hwnd: Handle, message: *const u16, title: *const u16, flags: u32) -> i32;
    fn SetWindowLongPtrW(hwnd: Handle, index: i32, value: isize) -> isize;
    fn GetWindowLongPtrW(hwnd: Handle, index: i32) -> isize;
}
#[link(name = "kernel32")]
extern "system" {
    fn GetModuleHandleW(name: *const u16) -> Handle;
}
#[link(name = "gdi32")]
extern "system" {
    fn CreateSolidBrush(color: u32) -> Handle;
    fn CreateFontW(height: i32, width: i32, escapement: i32, orientation: i32,
        weight: i32, italic: u32, underline: u32, strikeout: u32, charset: u32,
        output_precision: u32, clip_precision: u32, quality: u32,
        pitch_family: u32, face: *const u16) -> Handle;
    fn DeleteObject(object: Handle) -> i32;
    fn SelectObject(dc: Handle, object: Handle) -> Handle;
    fn SetBkMode(dc: Handle, mode: i32) -> i32;
    fn SetTextColor(dc: Handle, color: u32) -> u32;
    fn SetBkColor(dc: Handle, color: u32) -> u32;
}

fn wide(value: &str) -> Vec<u16> { value.encode_utf16().chain(std::iter::once(0)).collect() }
fn rgb(r: u32, g: u32, b: u32) -> u32 { r | (g << 8) | (b << 16) }
unsafe fn make_palette(colors: [(u32,u32,u32); 6]) -> Palette {
    let [surface, header, border, hover, ink, muted] = colors.map(|(r,g,b)| rgb(r,g,b));
    Palette {
        surface: CreateSolidBrush(surface), header: CreateSolidBrush(header),
        border: CreateSolidBrush(border), hover: CreateSolidBrush(hover),
        surface_color: surface, ink, muted,
    }
}
fn palette(theme: usize) -> &'static Palette { &PALETTES.get().unwrap()[theme] }
fn font() -> Handle { *FONT.get().unwrap() }
fn lo(value: isize) -> i32 { (value as u16 as i16) as i32 }
fn hi(value: isize) -> i32 { ((value >> 16) as u16 as i16) as i32 }

fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let count = match args.as_slice() {
        [] => 0,
        [flag, value] if flag == "--notes" =>
            match value.parse::<usize>() { Ok(n) if (1..=100).contains(&n) => n, _ => { usage(); return; } },
        _ => { usage(); return; },
    };
    unsafe {
        if let Err(error) = run(count) {
            MessageBoxW(0, wide(&error).as_ptr(), wide("Pinny Rust").as_ptr(), 0x10);
            std::process::exit(1);
        }
    }
}
fn usage() {
    unsafe { MessageBoxW(0, wide("Usage: Pinny.Rust.exe [--notes COUNT]").as_ptr(),
        wide("Pinny Rust").as_ptr(), 0x10); }
}
unsafe fn run(count: usize) -> Result<(), String> {
    let name = wide(CLASS);
    let instance = GetModuleHandleW(null());
    // Resource group #1 is added to the release executable by Build-Native.ps1.
    let large_icon = LoadImageW(instance, 1usize as *const u16, 1, 32, 32, 0);
    let small_icon = LoadImageW(instance, 1usize as *const u16, 1, 16, 16, 0);
    let class = WndClassEx {
        size: size_of::<WndClassEx>() as u32, style: 0, proc: Some(wnd_proc),
        cls_extra: 0, wnd_extra: 0, instance, icon: large_icon,
        cursor: LoadCursorW(0, 32512usize as *const u16),
        background: 0, menu_name: null(), class_name: name.as_ptr(), small_icon,
    };
    if RegisterClassExW(&class) == 0 { return Err("Could not register Win32 class".into()); }
    PALETTES.set([
        make_palette([(255,255,255),(244,245,247),(200,205,211),(225,229,235),(32,35,40),(85,91,99)]),
        make_palette([(36,39,44),(48,52,59),(76,84,94),(67,73,82),(232,234,237),(189,197,205)]),
        make_palette([(255,248,230),(235,218,183),(198,177,133),(222,201,161),(62,51,39),(107,88,59)]),
    ]).map_err(|_| "Could not initialize palettes")?;
    FONT.set(CreateFontW(-16,0,0,0,400,0,0,0,1,0,0,5,0,wide("Segoe UI").as_ptr()))
        .map_err(|_| "Could not initialize font")?;

    let result = (|| -> Result<(), String> {
        if count == 0 {
            show_note(InitialNote { x: i32::MIN, y: i32::MIN, width: 320, height: 320,
                text: String::new(), theme: 0, pinned: false })?;
        } else {
            for i in 0..count {
                let offset = ((i % 5) * 32) as i32;
                show_note(InitialNote { x: 80+offset, y: 80+offset, width: 320, height: 320,
                    text: format!("Benchmark note {i}\r\nA short plain-text note for comparison."),
                    theme: 0, pinned: false })?;
            }
        }
        let mut message: Msg = zeroed();
        while GetMessageW(&mut message, 0, 0, 0) > 0 {
            TranslateMessage(&message);
            DispatchMessageW(&message);
        }
        Ok(())
    })();
    for p in PALETTES.get().unwrap() {
        DeleteObject(p.surface); DeleteObject(p.header);
        DeleteObject(p.border); DeleteObject(p.hover);
    }
    DeleteObject(font());
    if large_icon != 0 { DestroyIcon(large_icon); }
    if small_icon != 0 { DestroyIcon(small_icon); }
    result
}
unsafe fn show_note(input: InitialNote) -> Result<(), String> {
    let width = input.width.clamp(210, 10000);
    let height = input.height.clamp(120, 10000);
    let sx = GetSystemMetrics(76); let sy = GetSystemMetrics(77);
    let sw = GetSystemMetrics(78); let sh = GetSystemMetrics(79);
    let mut x = if input.x == i32::MIN { sx+(sw-width)/2 } else { input.x };
    let mut y = if input.y == i32::MIN { sy+(sh-height)/2 } else { input.y };
    x = x.clamp(sx-width+48, sx+sw-48);
    y = y.clamp(sy, sy+sh-48);
    let hwnd = CreateWindowExW(WS_EX_APPWINDOW, wide(CLASS).as_ptr(), wide("Pinny").as_ptr(),
        WS_POPUP|WS_THICKFRAME|WS_SYSMENU|WS_MINIMIZEBOX, x,y,width,height,0,0,
        GetModuleHandleW(null()),null());
    if hwnd == 0 { return Err("Could not create note window".into()); }
    let note = Box::new(Note { edit: 0, menu: 0, theme: input.theme, pinned: input.pinned,
        hovered: 0, pressed: 0, scrollbar: false });
    let note = Box::into_raw(note);
    SetWindowLongPtrW(hwnd, GWLP_USERDATA, note as isize);
    WINDOWS.lock().unwrap().push(hwnd);
    (*note).menu = CreatePopupMenu();
    AppendMenuW((*note).menu, 0, LIGHT_ID, wide("Light").as_ptr());
    AppendMenuW((*note).menu, 0, DARK_ID, wide("Dark").as_ptr());
    AppendMenuW((*note).menu, 0, PAPER_ID, wide("Paper").as_ptr());
    AppendMenuW((*note).menu, MF_SEPARATOR, 0, null());
    AppendMenuW((*note).menu, 0, QUIT_ID, wide("Quit Pinny").as_ptr());
    (*note).edit = CreateWindowExW(0, wide("EDIT").as_ptr(), wide(&input.text).as_ptr(),
        WS_CHILD|WS_VISIBLE|WS_VSCROLL|ES_MULTILINE|ES_AUTOVSCROLL|ES_WANTRETURN,
        1,HEADER,width-2,height-HEADER-1,hwnd,EDIT_ID as isize,0,null());
    if (*note).edit == 0 { DestroyWindow(hwnd); return Err("Could not create note editor".into()); }
    SendMessageW((*note).edit, EM_LIMITTEXT, i32::MAX as usize, 0);
    SendMessageW((*note).edit, EM_SETMARGINS, 3, 10 | (10 << 16));
    SendMessageW((*note).edit, WM_SETFONT, font() as usize, 1);
    resize_edit(hwnd, note);
    ShowScrollBar((*note).edit, 1, 0);
    update_scrollbar(note);
    ShowWindow(hwnd, 5);
    if (*note).pinned { SetWindowPos(hwnd, -1,0,0,0,0,SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE); }
    UpdateWindow(hwnd);
    SetFocus((*note).edit);
    Ok(())
}
unsafe fn resize_edit(hwnd: Handle, note: *mut Note) {
    if (*note).edit == 0 { return; }
    let mut area: RECT = zeroed();
    GetClientRect(hwnd, &mut area);
    MoveWindow((*note).edit, 1, HEADER+8, (area.right-2).max(1),
        (area.bottom-HEADER-16).max(1), 1);
    update_scrollbar(note);
}
unsafe fn update_scrollbar(note: *mut Note) {
    if (*note).edit == 0 { return; }
    let mut area: RECT = zeroed();
    GetClientRect((*note).edit, &mut area);
    let lines = SendMessageW((*note).edit, EM_GETLINECOUNT, 0, 0);
    let show = lines*16 > area.bottom as isize;
    if show != (*note).scrollbar {
        ShowScrollBar((*note).edit, 1, show as i32);
        (*note).scrollbar = show;
    }
}
unsafe fn button_at(hwnd: Handle, lp: isize) -> usize {
    let x = lo(lp); let y = hi(lp);
    if !(0..HEADER).contains(&y) { return 0; }
    let mut area: RECT = zeroed();
    GetClientRect(hwnd, &mut area);
    if x >= area.right-36 { 4 } else if x >= area.right-72 { 3 }
    else if x >= area.right-117 { 2 } else if x >= area.right-151 { 1 } else { 0 }
}
unsafe extern "system" fn wnd_proc(hwnd: Handle, message: u32, wp: usize, lp: isize) -> isize {
    if message == WM_NCCALCSIZE { return 0; }
    let note = GetWindowLongPtrW(hwnd, GWLP_USERDATA) as *mut Note;
    if note.is_null() { return DefWindowProcW(hwnd, message, wp, lp); }
    match message {
        WM_NCHITTEST => {
            let mut pt = Point { x: lo(lp), y: hi(lp) };
            ScreenToClient(hwnd, &mut pt);
            let mut area: RECT = zeroed();
            GetClientRect(hwnd, &mut area);
            let ex = if pt.x < EDGE { -1 } else if pt.x >= area.right-EDGE { 1 } else { 0 };
            let ey = if pt.y < EDGE { -1 } else if pt.y >= area.bottom-EDGE { 1 } else { 0 };
            return match (ex,ey) {
                (-1,-1) => 13, (1,-1) => 14, (-1,1) => 16, (1,1) => 17,
                (-1,0) => 10, (1,0) => 11, (0,-1) => 12, (0,1) => 15, _ => 1,
            };
        }
        WM_ERASEBKGND => return 1,
        WM_PAINT => { paint_header(hwnd, note); return 0; }
        WM_MOUSEMOVE => {
            let hover = button_at(hwnd, lp);
            if hover != (*note).hovered {
                (*note).hovered = hover;
                InvalidateRect(hwnd, null(), 0);
            }
            let mut event = MouseTrack { size: size_of::<MouseTrack>() as u32,
                flags: 2, hwnd, hover_time: 0 };
            TrackMouseEvent(&mut event);
            return 0;
        }
        WM_MOUSELEAVE => { (*note).hovered = 0; InvalidateRect(hwnd,null(),0); return 0; }
        WM_LBUTTONDOWN => {
            (*note).pressed = button_at(hwnd, lp);
            if (*note).pressed != 0 { SetCapture(hwnd); }
            else if hi(lp) < HEADER {
                ReleaseCapture();
                SendMessageW(hwnd, WM_NCLBUTTONDOWN, 2, 0);
            }
            return 0;
        }
        WM_LBUTTONUP => {
            ReleaseCapture();
            let pressed = (*note).pressed;
            (*note).pressed = 0;
            if pressed != 0 && pressed == button_at(hwnd, lp) { click_button(hwnd, note, pressed); }
            return 0;
        }
        WM_SIZE => { resize_edit(hwnd, note); return 0; }
        WM_MOVE => return 0,
        WM_SETFOCUS => { if (*note).edit != 0 { SetFocus((*note).edit); } return 0; }
        WM_COMMAND => {
            let id = wp & 0xffff;
            let notification = (wp >> 16) & 0xffff;
            if id == EDIT_ID && notification == 0x300 { update_scrollbar(note); }
            else if id == NEW_ID { new_note(hwnd, note); }
            else if id == PIN_ID { toggle_pin(hwnd, note); }
            else if id == DELETE_ID { SendMessageW(hwnd, WM_CLOSE, 0, 0); }
            else if id == QUIT_ID { quit(); }
            else if [LIGHT_ID,DARK_ID,PAPER_ID].contains(&id) {
                (*note).theme = if id == DARK_ID { 1 } else if id == PAPER_ID { 2 } else { 0 };
                InvalidateRect((*note).edit, null(), 1);
                InvalidateRect(hwnd, null(), 0);
            }
            return 0;
        }
        WM_CTLCOLOREDIT => {
            let p = palette((*note).theme);
            SetTextColor(wp as isize, p.ink);
            SetBkColor(wp as isize, p.surface_color);
            return p.surface;
        }
        WM_CLOSE => { DestroyWindow(hwnd); return 0; }
        WM_DESTROY => {
            DestroyMenu((*note).menu);
            let mut handles = WINDOWS.lock().unwrap();
            handles.retain(|&h| h != hwnd);
            let empty = handles.is_empty();
            drop(handles);
            if empty { PostQuitMessage(0); }
            return 0;
        }
        WM_NCDESTROY => {
            SetWindowLongPtrW(hwnd, GWLP_USERDATA, 0);
            drop(Box::from_raw(note));
            return DefWindowProcW(hwnd, message, wp, lp);
        }
        _ => {}
    }
    DefWindowProcW(hwnd, message, wp, lp)
}
unsafe fn new_note(hwnd: Handle, note: *mut Note) {
    let mut bounds: RECT = zeroed();
    GetWindowRect(hwnd, &mut bounds);
    let _ = show_note(InitialNote {
        x: bounds.left+32, y: bounds.top+32,
        width: bounds.right-bounds.left, height: bounds.bottom-bounds.top,
        text: String::new(), theme: (*note).theme, pinned: (*note).pinned,
    });
}
unsafe fn toggle_pin(hwnd: Handle, note: *mut Note) {
    (*note).pinned = !(*note).pinned;
    SetWindowPos(hwnd, if (*note).pinned {-1} else {-2}, 0,0,0,0,
        SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE);
    InvalidateRect(hwnd, null(), 0);
}
unsafe fn click_button(hwnd: Handle, note: *mut Note, button: usize) {
    match button {
        1 => new_note(hwnd, note),
        2 => {
            let mut area: RECT = zeroed();
            GetClientRect(hwnd, &mut area);
            let mut pt = Point { x: area.right-117, y: HEADER };
            ClientToScreen(hwnd, &mut pt);
            SetForegroundWindow(hwnd);
            let choice = TrackPopupMenu((*note).menu, TPM_RETURNCMD, pt.x, pt.y, 0, hwnd, null());
            if choice != 0 { SendMessageW(hwnd, WM_COMMAND, choice as usize, 0); }
        }
        3 => toggle_pin(hwnd, note),
        4 => { SendMessageW(hwnd, WM_CLOSE, 0, 0); }
        _ => {}
    }
}
unsafe fn quit() {
    let handles = WINDOWS.lock().unwrap().clone();
    for hwnd in handles { DestroyWindow(hwnd); }
}
unsafe fn draw(dc: Handle, label: &str, mut bounds: RECT, format: u32, color: u32) {
    SetTextColor(dc, color);
    let chars = wide(label);
    DrawTextW(dc, chars.as_ptr(), (chars.len()-1) as i32, &mut bounds, format);
}
unsafe fn paint_header(hwnd: Handle, note: *mut Note) {
    let mut paint: PaintStruct = zeroed();
    let dc = BeginPaint(hwnd, &mut paint);
    if dc == 0 { return; }
    let mut bounds: RECT = zeroed();
    GetClientRect(hwnd, &mut bounds);
    let p = palette((*note).theme);
    FillRect(dc, &bounds, p.surface);
    let header = RECT { left: 0, top: 0, right: bounds.right, bottom: HEADER };
    let divider = RECT { left: 0, top: HEADER-1, right: bounds.right, bottom: HEADER };
    FillRect(dc, &header, p.header);
    FillRect(dc, &divider, p.border);
    FrameRect(dc, &bounds, p.border);
    SetBkMode(dc, 1);
    let old = SelectObject(dc, font());
    let title = RECT { left: 10, top: 1, right: (bounds.right-151).max(10), bottom: HEADER-1 };
    draw(dc, "Pinny", title, DT_VCENTER|DT_SINGLELINE, p.muted);
    let labels = ["+", "Menu", if (*note).pinned { "◆" } else { "◇" }, "×"];
    let widths = [34,45,36,36];
    let mut left = bounds.right-151;
    for (i, label) in labels.iter().enumerate() {
        let button = RECT { left, top: 1, right: left+widths[i], bottom: HEADER-1 };
        if (*note).hovered == i+1 { FillRect(dc, &button, p.hover); }
        draw(dc, label, button, DT_CENTER|DT_VCENTER|DT_SINGLELINE, p.ink);
        left += widths[i];
    }
    SelectObject(dc, old);
    EndPaint(hwnd, &paint);
}
