using Pinny;

namespace Pinny.WinFormsVariant;

internal static class Program
{
    [STAThread]
    private static void Main(string[] args)
    {
        ApplicationConfiguration.Initialize();
        NoteStorage.SetDataDirectory(args is ["--data-dir", var directory]
            ? directory : Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Pinny.Baseline", "WinForms"));
        try { Application.Run(new NotesContext()); }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException or System.Text.Json.JsonException)
        {
            MessageBox.Show($"Could not load notes. Saved data was not changed.\n\n{ex.Message}",
                "Pinny", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
    }
}
