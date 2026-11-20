package main

// The C file beside this one exports the variables that ask a laptop's
// drivers for its dedicated graphics chip; see gpu_windows.c. Importing C is
// what has it compiled and linked into the executable.

import "C"
