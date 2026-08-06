param(
  [Parameter(Mandatory = $true)] [string] $Reference,
  [Parameter(Mandatory = $true)] [string] $Candidate,
  [Parameter(Mandatory = $false)] [string] $ReportPath
)

Add-Type -AssemblyName System.Drawing
$systemDrawingAssembly = [System.Drawing.Bitmap].Assembly.Location

# GetPixel is convenient for tiny dialogs but becomes prohibitively slow for
# the 1936x1048 legacy shell. Compare locked 32bpp buffers instead so the
# same gate can be used for full-window captures without timing out.
Add-Type -ReferencedAssemblies $systemDrawingAssembly -TypeDefinition @'
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;

public sealed class PngPixelComparison {
  public long DifferentPixels { get; set; }
  public int MaxChannelDelta { get; set; }
}

public static class PngPixelComparer {
  public static PngPixelComparison Compare(Bitmap left, Bitmap right) {
    using (var a = ToArgb(left))
    using (var b = ToArgb(right)) {
      var rect = new Rectangle(0, 0, a.Width, a.Height);
      var ad = a.LockBits(rect, ImageLockMode.ReadOnly, PixelFormat.Format32bppArgb);
      var bd = b.LockBits(rect, ImageLockMode.ReadOnly, PixelFormat.Format32bppArgb);
      try {
        var aBytes = new byte[Math.Abs(ad.Stride) * a.Height];
        var bBytes = new byte[Math.Abs(bd.Stride) * b.Height];
        Marshal.Copy(ad.Scan0, aBytes, 0, aBytes.Length);
        Marshal.Copy(bd.Scan0, bBytes, 0, bBytes.Length);
        long different = 0;
        int maxDelta = 0;
        for (var y = 0; y < a.Height; y++) {
          var aRow = (ad.Stride >= 0 ? y : a.Height - 1 - y) * Math.Abs(ad.Stride);
          var bRow = (bd.Stride >= 0 ? y : b.Height - 1 - y) * Math.Abs(bd.Stride);
          for (var x = 0; x < a.Width; x++) {
            var offset = x * 4;
            var pixelDifferent = false;
            for (var channel = 0; channel < 4; channel++) {
              var delta = Math.Abs(aBytes[aRow + offset + channel] - bBytes[bRow + offset + channel]);
              if (delta > 0) pixelDifferent = true;
              if (delta > maxDelta) maxDelta = delta;
            }
            if (pixelDifferent) different++;
          }
        }
        return new PngPixelComparison { DifferentPixels = different, MaxChannelDelta = maxDelta };
      }
      finally {
        a.UnlockBits(ad);
        b.UnlockBits(bd);
      }
    }
  }

  private static Bitmap ToArgb(Bitmap source) {
    var copy = new Bitmap(source.Width, source.Height, PixelFormat.Format32bppArgb);
    using (var graphics = Graphics.FromImage(copy)) graphics.DrawImageUnscaled(source, 0, 0);
    return copy;
  }
}
'@

$referenceBitmap = [System.Drawing.Bitmap]::new([System.IO.Path]::GetFullPath($Reference))
$candidateBitmap = [System.Drawing.Bitmap]::new([System.IO.Path]::GetFullPath($Candidate))
try {
  if ($referenceBitmap.Width -ne $candidateBitmap.Width -or $referenceBitmap.Height -ne $candidateBitmap.Height) {
    throw "Image dimensions differ: reference $($referenceBitmap.Width)x$($referenceBitmap.Height), candidate $($candidateBitmap.Width)x$($candidateBitmap.Height)."
  }

  $comparison = [PngPixelComparer]::Compare($referenceBitmap, $candidateBitmap)
  $differentPixels = [int64]$comparison.DifferentPixels
  $maxChannelDelta = [int]$comparison.MaxChannelDelta

  $result = [pscustomobject]@{
    reference = [System.IO.Path]::GetFullPath($Reference)
    candidate = [System.IO.Path]::GetFullPath($Candidate)
    width = $referenceBitmap.Width
    height = $referenceBitmap.Height
    totalPixels = $referenceBitmap.Width * $referenceBitmap.Height
    differentPixels = $differentPixels
    maxChannelDelta = $maxChannelDelta
    result = if ($differentPixels -eq 0) { 'pass' } else { 'fail' }
  }
  $json = $result | ConvertTo-Json -Depth 3
  if ($ReportPath) {
    $resolvedReport = [System.IO.Path]::GetFullPath($ReportPath)
    $parent = [System.IO.Path]::GetDirectoryName($resolvedReport)
    if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    [System.IO.File]::WriteAllText($resolvedReport, $json, [System.Text.UTF8Encoding]::new($false))
  }
  $json
  if ($differentPixels -ne 0) { exit 1 }
} finally {
  $referenceBitmap.Dispose()
  $candidateBitmap.Dispose()
}
