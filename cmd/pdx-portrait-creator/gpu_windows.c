// A laptop with a graphics chip of its own besides the one in its processor
// runs a program on the slower, built in one unless the program asks for the
// other, which the portrait's shaders are far too heavy for. The drivers of
// NVIDIA Optimus and AMD switchable graphics look for these two variables
// exported from the executable, and run it on the dedicated chip when they
// are set.
__declspec(dllexport) unsigned long NvOptimusEnablement = 1;
__declspec(dllexport) int AmdPowerXpressRequestHighPerformance = 1;
