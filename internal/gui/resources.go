package gui

// The Windows resources of the application, compiled from resources.rc into
// rsrc_windows_amd64.syso, which Go links into every Windows binary that uses
// this package: the application and its interface tests alike.
//
// They carry the application icon, which Explorer and the task bar show for
// the executable, and the application manifest, whose one setting that
// matters is the heap. Windows gives applications installed from the Store
// the segment heap, which places large blocks at the very end of their memory
// pages, so reading even a byte past one crashes at once. The ordinary heap
// leaves some memory there, and the same bug goes unnoticed. raylib's DDS
// reader had exactly such a bug, which only ever crashed the Store version.
// With the segment heap in every build, a bug like it crashes in development
// and in the tests instead.
//
// icon.ico holds the Store's square tiles of 16, 24, 32, 48 and 256 pixels,
// packaging/msix/Assets/PDXPORTRAITCREATOR-Square44x44Logo.targetsize-*.png,
// each as it is, a PNG. After changing icon.ico, windows.manifest or
// resources.rc, regenerate the .syso with MinGW's windres:

//go:generate windres -i resources.rc -O coff -o rsrc_windows_amd64.syso
