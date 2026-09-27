namespace Pinny;

public sealed class NoteState
{
    public Guid Id { get; set; } = Guid.NewGuid();
    public string Text { get; set; } = string.Empty;
    public double Left { get; set; } = double.NaN;
    public double Top { get; set; } = double.NaN;
    public double Width { get; set; } = 320;
    public double Height { get; set; } = 320;
    public bool IsPinned { get; set; }
    public string Theme { get; set; } = "Light";
}
