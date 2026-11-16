#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

use rune::{
    AppSpec, Color, Cx, El, ScrollSpec, TextEdit, TextStyle, WindowHandle, WindowOptions,
    WindowSpec, col, el, row, text, text_input,
};
use serde::Deserialize;
use std::{
    path::PathBuf,
    sync::atomic::{AtomicUsize, Ordering},
};

static NEXT_WINDOW: AtomicUsize = AtomicUsize::new(0);

#[derive(Deserialize)]
#[serde(rename_all = "PascalCase")]
struct InputNote {
    text: String,
    left: f64,
    top: f64,
    width: f64,
    height: f64,
    #[serde(default)]
    is_pinned: bool,
    #[serde(default)]
    theme: String,
}

struct Note {
    edit: TextEdit,
    pinned: bool,
    menu: bool,
    theme: usize,
    window: Option<WindowHandle>,
}

fn color(r: u8, g: u8, b: u8) -> Color {
    Color {
        r: rune::color::srgb_decode(r as f32 / 255.0),
        g: rune::color::srgb_decode(g as f32 / 255.0),
        b: rune::color::srgb_decode(b as f32 / 255.0),
        a: 1.0,
    }
}

fn palette(theme: usize) -> (Color, Color, Color) {
    match theme {
        1 => (color(36, 39, 44), color(48, 52, 59), color(232, 234, 237)),
        2 => (
            color(255, 248, 230),
            color(235, 218, 183),
            color(62, 51, 39),
        ),
        _ => (Color::WHITE, color(244, 245, 247), color(32, 35, 40)),
    }
}

fn button(label: &'static str, key: &'static str, ink: Color) -> El {
    el().key(key)
        .role("button")
        .label(label)
        .focusable(true)
        .w(if label == "Menu" { 45.0 } else { 34.0 })
        .h(30.0)
        .center()
        .child(text(label).font(TextStyle::new(12.0, 400.0)).color(ink))
}

fn view(s: &Note, _: &mut Cx<Note>) -> El {
    let (surface, header, ink) = palette(s.theme);
    let bar = row()
        .w_full()
        .h(32.0)
        .shrink()
        .fill(header)
        .window_drag()
        .child(
            el().grow(1.0)
                .padding(10.0)
                .child(text("Pinny").font(TextStyle::new(12.0, 400.0)).color(ink)),
        )
        .child(button("+", "new", ink).on_press(|_: &mut Note| {
            rune::open_window(window(InputNote {
                text: String::new(),
                left: 160.0,
                top: 160.0,
                width: 320.0,
                height: 320.0,
                is_pinned: false,
                theme: "Light".into(),
            }));
        }))
        .child(button("Menu", "menu", ink).on_press(|s: &mut Note| s.menu = !s.menu))
        .child(
            button(if s.pinned { "◆" } else { "◇" }, "pin", ink).on_press(|s: &mut Note| {
                s.pinned = !s.pinned;
                if let Some(window) = &s.window {
                    window.set_always_on_top(s.pinned);
                }
            }),
        )
        .child(button("×", "close", ink).on_press_tx(|_: &mut Note, tx| tx.close_window()));
    let editor = text_input(&s.edit, TextStyle::new(16.0, 400.0).lh(20.0))
        .key("editor")
        .label("Note text")
        .w_full()
        .min_h(0.0)
        .color(ink)
        .on_edit(|s: &mut Note| &mut s.edit)
        .on_submit(|s: &mut Note, _| s.edit.insert("\n"));
    let mut content = col()
        .w_full()
        .h_full()
        .fill(surface)
        .border(1.0, color(200, 205, 211))
        .child(bar)
        .child(
            col()
                .key("scroll")
                .grow(1.0)
                .min_h(0.0)
                .w_full()
                .padding(10.0)
                .scroll(ScrollSpec {
                    stick_bottom: false,
                })
                .child(editor),
        );
    if s.menu {
        let mut menu = col()
            .key("popup")
            .absolute(120.0, 32.0)
            .w(160.0)
            .fill(header)
            .padding(6.0);
        for (index, label) in ["Light", "Dark", "Paper"].into_iter().enumerate() {
            menu = menu.child(
                button(label, label, ink)
                    .w_full()
                    .on_press(move |s: &mut Note| {
                        s.theme = index;
                        s.menu = false;
                    }),
            );
        }
        menu = menu.child(
            button("Quit Pinny", "quit", ink)
                .w_full()
                .on_press_tx(|_: &mut Note, tx| tx.quit()),
        );
        content = content.child(menu);
    }
    content
}

fn blank() -> Note {
    Note {
        edit: TextEdit::new(""),
        pinned: false,
        menu: false,
        theme: 0,
        window: None,
    }
}

fn window(input: InputNote) -> WindowSpec<Note> {
    let id = NEXT_WINDOW.fetch_add(1, Ordering::Relaxed);
    let theme = match input.theme.as_str() {
        "Dark" => 1,
        "Paper" => 2,
        _ => 0,
    };
    let state = Note {
        edit: TextEdit::new(input.text.replace("\r\n", "\n")),
        pinned: input.is_pinned,
        theme,
        ..blank()
    };
    let spec = AppSpec {
        min_size: Some((210.0, 120.0)),
        families: ("Segoe UI", "Consolas"),
        ..AppSpec::new(
            "Pinny",
            (input.width as f32, input.height as f32),
            blank,
            view,
        )
    };
    let app = rune::App::new(spec).init(move || state);
    let handle = app.handle();
    WindowSpec::from_app(format!("note-{id}"), app)
        .options(WindowOptions {
            borderless: true,
            position: Some((input.left as i32, input.top as i32)),
            always_on_top: input.is_pinned,
            ..Default::default()
        })
        .on_created(move |window| handle.update(move |s, _| s.window = Some(window.clone())))
}

fn inputs(args: &[String]) -> Result<Vec<InputNote>, String> {
    let mut directory = None;
    let mut count = None;
    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "--data-dir" => {
                i += 1;
                directory = Some(PathBuf::from(
                    args.get(i).ok_or("--data-dir requires a path")?,
                ));
            }
            "--notes" => {
                i += 1;
                let n = args
                    .get(i)
                    .ok_or("--notes requires a count")?
                    .parse::<usize>()
                    .map_err(|_| "Invalid note count")?;
                if !(1..=100).contains(&n) {
                    return Err("--notes must be between 1 and 100".into());
                }
                count = Some(n);
            }
            _ => {}
        }
        i += 1;
    }
    if directory.is_some() && count.is_some() {
        return Err("Choose --data-dir or --notes".into());
    }
    if let Some(path) = directory {
        let bytes = std::fs::read(path.join("notes.json")).map_err(|e| e.to_string())?;
        let bytes = bytes.strip_prefix(&[0xef, 0xbb, 0xbf]).unwrap_or(&bytes);
        let notes: Vec<InputNote> = serde_json::from_slice(bytes).map_err(|e| e.to_string())?;
        if notes.is_empty() || notes.len() > 100 {
            return Err("Expected 1 to 100 notes".into());
        }
        for note in &notes {
            if ![note.left, note.top, note.width, note.height]
                .iter()
                .all(|n| n.is_finite())
                || !(210.0..=10000.0).contains(&note.width)
                || !(120.0..=10000.0).contains(&note.height)
            {
                return Err("Invalid note geometry".into());
            }
        }
        return Ok(notes);
    }
    Ok((0..count.unwrap_or(1))
        .map(|i| InputNote {
            text: if count.is_some() {
                format!("Benchmark note {i}\r\nA short plain-text note for comparison.")
            } else {
                String::new()
            },
            left: (80 + (i % 5) * 32) as f64,
            top: (80 + (i % 5) * 32) as f64,
            width: 320.0,
            height: 320.0,
            is_pinned: false,
            theme: "Light".into(),
        })
        .collect())
}

fn main() {
    let args: Vec<_> = std::env::args().skip(1).collect();
    let notes = match inputs(&args) {
        Ok(notes) => notes,
        Err(error) => {
            eprintln!("Pinny Rune: {error}");
            std::process::exit(2);
        }
    };
    // Rune receives only its own tooling flags; benchmark input flags are handled above.
    let mut tooling = Vec::new();
    let mut i = 0;
    while i < args.len() {
        if args[i] == "--data-dir" || args[i] == "--notes" {
            i += 2;
        } else {
            tooling.push(args[i].clone());
            i += 1;
        }
    }
    rune::run_windows_with_args(rune::RunOptions::default(), tooling, move |_| {
        notes.into_iter().map(window).collect::<Vec<_>>()
    });
}
