package app

/*
#cgo windows LDFLAGS: -lopengl32
#cgo linux LDFLAGS: -lGL
#include <GL/gl.h>

static const char *rendererName(void) {
	return (const char *)glGetString(GL_RENDERER);
}
*/
import "C"

// graphicsName is the name the driver gives the graphics chip the window
// draws with, such as Intel(R) UHD Graphics, which the warning of a slow
// portrait names. It needs the OpenGL context; raylib, which would say it,
// has had its log turned down to errors by the time the window is open.
func graphicsName() string {
	name := C.rendererName()
	if name == nil {
		return "an unknown graphics chip"
	}

	return C.GoString(name)
}
