param(
  [Parameter(Mandatory = $false)] [int] $ProcessId,
  [Parameter(Mandatory = $false)] [long] $WindowHandle,
  [Parameter(Mandatory = $true)] [string] $OutputPath
)

Add-Type -AssemblyName System.Drawing

if (-not ([System.Management.Automation.PSTypeName]'AbuzarNext.Win32Capture').Type) {
  Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;

namespace AbuzarNext {
  public static class Win32Capture {
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left; public int Top; public int Right; public int Bottom; }

    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);

    [DllImport("user32.dll")]
    public static extern bool IsWindowVisible(IntPtr hWnd);

    [DllImport("user32.dll")]
    public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdcBlt, uint nFlags);
  }
}
'@
}

$process = $null
$hWnd = [IntPtr]::Zero
if ($WindowHandle) {
  $hWnd = [IntPtr]::new($WindowHandle)
  $processIdForCapture = 0
  $processIdForCapture = [System.Diagnostics.Process]::GetProcesses() |
    Where-Object { $_.MainWindowHandle -eq $hWnd } |
    Select-Object -First 1 -ExpandProperty Id
  if ($processIdForCapture) { $process = Get-Process -Id $processIdForCapture -ErrorAction SilentlyContinue }
} else {
  if (-not $ProcessId) { throw 'Specify either ProcessId or WindowHandle.' }
  $process = Get-Process -Id $ProcessId -ErrorAction Stop
  $hWnd = $process.MainWindowHandle
}

if ($hWnd -eq [IntPtr]::Zero) { throw 'The supplied process/window has no usable window handle.' }

$rect = [AbuzarNext.Win32Capture+RECT]::new()
if (-not [AbuzarNext.Win32Capture]::GetWindowRect($hWnd, [ref] $rect)) {
  throw 'Unable to resolve the supplied window rectangle.'
}

$width = $rect.Right - $rect.Left
$height = $rect.Bottom - $rect.Top
if ($width -le 0 -or $height -le 0) {
  throw "Window rectangle is empty: ${width}x${height}."
}

$resolvedOutput = [System.IO.Path]::GetFullPath($OutputPath)
$parent = [System.IO.Path]::GetDirectoryName($resolvedOutput)
if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }

$bitmap = [System.Drawing.Bitmap]::new($width, $height, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$hdc = $graphics.GetHdc()
try {
  $rendered = [AbuzarNext.Win32Capture]::PrintWindow($hWnd, $hdc, 2)
} finally {
  $graphics.ReleaseHdc($hdc)
  $graphics.Dispose()
}

if (-not $rendered) {
  $bitmap.Dispose()
  throw 'PrintWindow failed for the supplied window.'
}

try {
  $bitmap.Save($resolvedOutput, [System.Drawing.Imaging.ImageFormat]::Png)
} finally {
  $bitmap.Dispose()
}

[pscustomobject]@{
  processId = if ($process) { $process.Id } else { $null }
  processPath = if ($process) { $process.Path } else { $null }
  windowTitle = if ($process) { $process.MainWindowTitle } else { $null }
  windowHandle = ('0x{0:X}' -f $hWnd.ToInt64())
  left = $rect.Left
  top = $rect.Top
  width = $width
  height = $height
  output = $resolvedOutput
} | ConvertTo-Json -Depth 3
