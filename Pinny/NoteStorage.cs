using System.IO;
using System.Text.Json;

namespace Pinny;

public static class NoteStorage
{
    private static readonly string DirectoryPath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Pinny");

    private static readonly string FilePath = Path.Combine(DirectoryPath, "note.json");

    private static readonly JsonSerializerOptions JsonOptions = new() { WriteIndented = true };

    public static NoteState Load()
    {
        try
        {
            if (File.Exists(FilePath))
                return JsonSerializer.Deserialize<NoteState>(File.ReadAllText(FilePath)) ?? new NoteState();
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException or JsonException)
        {
            System.Diagnostics.Debug.WriteLine($"Could not load note: {exception}");
        }

        return new NoteState();
    }

    public static void Save(NoteState state)
    {
        Directory.CreateDirectory(DirectoryPath);
        string temporaryPath = FilePath + ".tmp";
        File.WriteAllText(temporaryPath, JsonSerializer.Serialize(state, JsonOptions));
        File.Move(temporaryPath, FilePath, overwrite: true);
    }
}
