Material Symbols Outlined
=========================

From https://fonts.google.com/icons, licensed under the Apache License 2.0;
LICENSE.txt is the licence as it was downloaded with the font.

MaterialSymbolsOutlined-Subset.ttf is Material Symbols Outlined cut down to the
icons the interface draws, at an optical size of 20, a weight of 400, unfilled,
which is why it is twelve kilobytes rather than ten megabytes. The whole font
holds some four thousand icons, and the application would carry every one of
them around for the sake of a few dozen.

Google Fonts cuts it down itself. Ask its stylesheet for the icons by name, in
alphabetical order, without a browser's user agent, which makes it answer with
a TrueType font rather than WOFF2:

    curl "https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:opsz,wght,FILL,GRAD@20,400,0,0&icon_names=<names>"

and download the font its src names. The names in it now:

    accessibility_new,add,add_circle,add_photo_alternate,air,animation,
    arrow_back,arrow_downward,arrow_forward,arrow_upward,boy,cake,category,
    content_copy,content_cut,content_paste,delete,elderly,eyebrow,face,face_5,face_6,female,fingerprint,fit_screen,
    girl,groups,hearing,highlight,info,light_mode,link,link_off,lips,logout,
    male,palette,pause,person,person_add,photo_camera,play_arrow,restart_alt,
    save,save_as,search,sentiment_satisfied,settings,visibility,warning,
    zoom_in,zoom_out

The interface asks for an icon by its code point, which internal/gui/fonts.go
names; the gallery gives the code point of each icon, and so does the list of
them beside the font in github.com/google/material-design-icons
(variablefont/*.codepoints).
