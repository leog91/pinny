using Pinny;

namespace Pinny.WinFormsVariant;

internal sealed class NotesContext : ApplicationContext
{
    private readonly List<NoteForm> _forms = [];
    private readonly System.Windows.Forms.Timer _timer = new() { Interval = 500 };
    private bool _quitting;
    private bool _saveErrorShown;

    public NotesContext()
    {
        _timer.Tick += (_, _) => { _timer.Stop(); TrySave(); };
        List<NoteState> notes = NoteStorage.LoadAll();
        if (notes.Count == 0) notes.Add(new NoteState());
        foreach (NoteState note in notes) ShowNote(note);
    }

    private void ShowNote(NoteState state)
    {
        var form = new NoteForm(this, state);
        _forms.Add(form);
        form.FormClosed += (_, _) => { _forms.Remove(form); if (_forms.Count == 0) ExitThread(); };
        form.Show();
    }

    public void NewNote(NoteForm source)
    {
        NoteState current = source.CaptureState();
        ShowNote(new NoteState
        {
            Left = current.Left + 32, Top = current.Top + 32,
            Width = current.Width, Height = current.Height,
            IsPinned = current.IsPinned, Theme = current.Theme
        });
        TrySave();
    }

    public void QueueSave() { _timer.Stop(); _timer.Start(); }
    public bool CloseNote(NoteForm form) { _timer.Stop(); return _quitting || TrySave(form); }

    public void Quit()
    {
        _timer.Stop();
        if (!TrySave()) return;
        _quitting = true;
        foreach (NoteForm form in _forms.ToArray()) form.Close();
    }

    private bool TrySave(NoteForm? excluded = null)
    {
        try
        {
            NoteStorage.SaveAll(_forms.Where(form => form != excluded).Select(form => form.CaptureState()));
            _saveErrorShown = false;
            return true;
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException or System.Text.Json.JsonException)
        {
            if (!_saveErrorShown)
            {
                _saveErrorShown = true;
                MessageBox.Show($"Could not save notes.\n\n{ex.Message}", "Pinny", MessageBoxButtons.OK, MessageBoxIcon.Warning);
            }
            return false;
        }
    }

    protected override void Dispose(bool disposing)
    {
        if (disposing) _timer.Dispose();
        base.Dispose(disposing);
    }
}
