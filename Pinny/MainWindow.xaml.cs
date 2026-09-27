using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Threading;

namespace Pinny;

public partial class MainWindow : Window
{
    private readonly DispatcherTimer _saveTimer = new() { Interval = TimeSpan.FromMilliseconds(500) };
    private bool _restoring = true;
    private bool _saveErrorShown;
    private string _theme = "Light";

    public MainWindow()
    {
        InitializeComponent();
        _saveTimer.Tick += SaveTimer_Tick;
        RestoreState();
        LocationChanged += WindowGeometryChanged;
        SizeChanged += WindowGeometryChanged;
        Closing += MainWindow_Closing;
        Loaded += (_, _) => NoteTextBox.Focus();
    }

    private void RestoreState()
    {
        _restoring = true;
        NoteState state = NoteStorage.Load();
        ApplyTheme(state.Theme);
        NoteTextBox.Text = state.Text ?? string.Empty;
        Width = double.IsFinite(state.Width) ? Math.Max(MinWidth, state.Width) : 320;
        Height = double.IsFinite(state.Height) ? Math.Max(MinHeight, state.Height) : 320;

        // Leave an off-screen note recoverable if the monitor layout changed.
        if (double.IsFinite(state.Left) && double.IsFinite(state.Top))
        {
            double visibleLeft = SystemParameters.VirtualScreenLeft;
            double visibleTop = SystemParameters.VirtualScreenTop;
            double visibleRight = visibleLeft + SystemParameters.VirtualScreenWidth;
            double visibleBottom = visibleTop + SystemParameters.VirtualScreenHeight;
            Left = Math.Clamp(state.Left, visibleLeft - Width + 48, visibleRight - 48);
            Top = Math.Clamp(state.Top, visibleTop, visibleBottom - 48);
        }
        else
        {
            Rect workArea = SystemParameters.WorkArea;
            Left = workArea.Left + (workArea.Width - Width) / 2;
            Top = workArea.Top + (workArea.Height - Height) / 2;
        }

        PinButton.IsChecked = state.IsPinned;
        Topmost = state.IsPinned;
        _restoring = false;
    }

    private void Header_MouseLeftButtonDown(object sender, MouseButtonEventArgs e)
    {
        if (e.OriginalSource is Button or System.Windows.Controls.Primitives.ToggleButton)
            return;

        if (e.LeftButton == MouseButtonState.Pressed)
            DragMove();
    }

    private void PinButton_Changed(object sender, RoutedEventArgs e)
    {
        if (PinButton is null)
            return;

        Topmost = PinButton.IsChecked == true;
        PinButton.Content = Topmost ? "◆" : "◇";
        QueueSave();
    }

    private void ThemeButton_Click(object sender, RoutedEventArgs e)
    {
        if (ThemeButton.ContextMenu is ContextMenu menu)
        {
            menu.PlacementTarget = ThemeButton;
            menu.IsOpen = true;
        }
    }

    private void ThemeMenuItem_Click(object sender, RoutedEventArgs e)
    {
        if (sender is MenuItem { Tag: string theme })
        {
            ApplyTheme(theme);
            QueueSave();
            NoteTextBox.Focus();
        }
    }

    private void ApplyTheme(string? theme)
    {
        _theme = theme is "Dark" or "Paper" ? theme : "Light";
        (string surface, string header, string border, string text, string muted, string selection) = _theme switch
        {
            "Dark" => ("#24272C", "#30343B", "#4C545E", "#E8EAED", "#BDC5CD", "#586F89"),
            "Paper" => ("#FFF8E6", "#EBDAB7", "#C6B185", "#3E3327", "#6B583B", "#EED28A"),
            _ => ("#FFFFFF", "#F4F5F7", "#C8CDD3", "#202328", "#555B63", "#A8CCF4")
        };

        static SolidColorBrush Brush(string color) => new((Color)ColorConverter.ConvertFromString(color)!);

        Background = Brush(surface);
        WindowBorder.Background = Brush(surface);
        WindowBorder.BorderBrush = Brush(border);
        Header.Background = Brush(header);
        TitleText.Foreground = Brush(muted);
        NoteTextBox.Background = Brush(surface);
        NoteTextBox.Foreground = Brush(text);
        NoteTextBox.CaretBrush = Brush(text);
        NoteTextBox.SelectionBrush = Brush(selection);

        foreach (Control button in new Control[] { ThemeButton, PinButton, CloseButton })
        {
            button.Background = Brush(header);
            button.Foreground = Brush(text);
            button.BorderBrush = Brush(header);
        }

        if (ThemeButton.ContextMenu is ContextMenu menu)
        {
            menu.Background = Brush(header);
            menu.Foreground = Brush(text);
        }

        foreach (MenuItem item in new[] { LightThemeItem, DarkThemeItem, PaperThemeItem })
        {
            item.Background = Brush(header);
            item.Foreground = Brush(text);
            item.IsChecked = (string)item.Tag == _theme;
        }

        ThemeButton.ToolTip = $"Theme: {_theme}";
    }
    private void CloseButton_Click(object sender, RoutedEventArgs e) => Close();

    private void NoteTextBox_TextChanged(object sender, TextChangedEventArgs e) => QueueSave();

    private void WindowGeometryChanged(object? sender, EventArgs e) => QueueSave();

    private void QueueSave()
    {
        if (_restoring)
            return;

        _saveTimer.Stop();
        _saveTimer.Start();
    }

    private void SaveTimer_Tick(object? sender, EventArgs e)
    {
        _saveTimer.Stop();
        SaveState();
    }

    private void MainWindow_Closing(object? sender, System.ComponentModel.CancelEventArgs e)
    {
        _saveTimer.Stop();
        e.Cancel = !SaveState();
    }

    private bool SaveState()
    {
        try
        {
            NoteStorage.Save(new NoteState
            {
                Text = NoteTextBox.Text,
                Left = Left,
                Top = Top,
                Width = ActualWidth,
                Height = ActualHeight,
                IsPinned = Topmost,
                Theme = _theme
            });
            _saveErrorShown = false;
            return true;
        }
        catch (Exception exception) when (exception is System.IO.IOException or UnauthorizedAccessException)
        {
            System.Diagnostics.Debug.WriteLine($"Could not save note: {exception}");
            if (!_saveErrorShown)
            {
                _saveErrorShown = true;
                MessageBox.Show(this, $"Pinny could not save your note.\n\n{exception.Message}",
                    "Save failed", MessageBoxButton.OK, MessageBoxImage.Warning);
            }
            return false;
        }
    }
}
