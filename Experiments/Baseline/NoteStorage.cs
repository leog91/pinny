using System.IO;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pinny;

public static class NoteStorage
{
    private static string DirectoryPath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Pinny");

    private static string NotesPath => Path.Combine(DirectoryPath, "notes.json");
    private static string LegacyNotePath => Path.Combine(DirectoryPath, "note.json");

    public static void SetDataDirectory(string directory) => DirectoryPath = directory;

    public static List<NoteState> LoadAll()
    {
        if (File.Exists(NotesPath))
        {
            List<NoteState> notes = JsonSerializer.Deserialize(
                File.ReadAllText(NotesPath), NoteJsonContext.Default.ListNoteState) ?? new List<NoteState>();
            return notes;
        }

        // Preserve the note created by earlier single-note versions.
        if (File.Exists(LegacyNotePath))
        {
            NoteState? oldNote = JsonSerializer.Deserialize(File.ReadAllText(LegacyNotePath), NoteJsonContext.Default.NoteState);
            return oldNote is null ? new List<NoteState>() : new List<NoteState> { oldNote };
        }

        return new List<NoteState>();
    }

    public static void SaveAll(IEnumerable<NoteState> notes)
    {
        Directory.CreateDirectory(DirectoryPath);
        string temporaryPath = NotesPath + ".tmp";
        File.WriteAllText(temporaryPath, JsonSerializer.Serialize(notes.ToList(), NoteJsonContext.Default.ListNoteState));
        File.Move(temporaryPath, NotesPath, overwrite: true);
    }
}

[JsonSourceGenerationOptions(WriteIndented = true)]
[JsonSerializable(typeof(List<NoteState>))]
[JsonSerializable(typeof(NoteState))]
internal partial class NoteJsonContext : JsonSerializerContext { }
