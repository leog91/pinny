using System.IO;
using System.Text.Json;
using System.Windows;

namespace Pinny;

public partial class App : Application
{
    private bool _isQuitting;
    private bool _saveErrorShown;

    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);
        ShutdownMode = ShutdownMode.OnExplicitShutdown;

        if (e.Args is ["--data-dir", var directory])
            NoteStorage.SetDataDirectory(directory);

        List<NoteState> notes;
        try
        {
            notes = NoteStorage.LoadAll();
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException or JsonException)
        {
            MessageBox.Show($"Pinny could not load your notes. The saved file was not changed.\n\n{exception.Message}",
                "Load failed", MessageBoxButton.OK, MessageBoxImage.Error);
            Shutdown();
            return;
        }

        if (notes.Count == 0)
        {
            CreateNote();
            return;
        }

        foreach (NoteState note in notes)
            new MainWindow(note).Show();
    }

    public void CreateNote(MainWindow? source = null)
    {
        var note = new NoteState();
        if (source is not null)
        {
            NoteState current = source.CaptureState();
            note.Left = current.Left + 32;
            note.Top = current.Top + 32;
            note.Width = current.Width;
            note.Height = current.Height;
            note.IsPinned = current.IsPinned;
            note.Theme = current.Theme;
        }

        var window = new MainWindow(note);
        window.Show();
        window.Activate();
        TrySaveAll();
    }

    public bool TryCloseNote(MainWindow note)
    {
        if (_isQuitting)
            return true;

        if (!TrySaveAll(note))
            return false;

        if (Windows.OfType<MainWindow>().Count() == 1)
            note.Closed += (_, _) => Shutdown();

        return true;
    }

    public void Quit()
    {
        if (!TrySaveAll())
            return;

        _isQuitting = true;
        Shutdown();
    }

    protected override void OnSessionEnding(SessionEndingCancelEventArgs e)
    {
        if (!TrySaveAll())
            e.Cancel = true;
        else
            _isQuitting = true;

        base.OnSessionEnding(e);
    }

    internal bool TrySaveAll(MainWindow? excluded = null)
    {
        try
        {
            NoteStorage.SaveAll(Windows.OfType<MainWindow>()
                .Where(window => window != excluded)
                .Select(window => window.CaptureState()));
            _saveErrorShown = false;
            return true;
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException or JsonException)
        {
            System.Diagnostics.Debug.WriteLine($"Could not save notes: {exception}");
            if (!_saveErrorShown)
            {
                _saveErrorShown = true;
                MessageBox.Show($"Pinny could not save your notes.\n\n{exception.Message}",
                    "Save failed", MessageBoxButton.OK, MessageBoxImage.Warning);
            }
            return false;
        }
    }
}
