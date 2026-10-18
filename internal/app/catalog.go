package app

import (
	"strings"
	"unicode"

	"github.com/kaiser-chris/pdx-portrait-creator/internal/gui"
)

// The catalogue sorts the genes into the categories the character panel
// lists them under, and gives each the name the panel shows it by. The game
// files name a gene only by its key, such as gene_forehead_brow_inner_height,
// and group them by nothing a person editing a face would look for, so both
// are kept here, for the genes of Victoria 3. A gene the catalogue does not
// know, such as one a mod adds, is listed under Other by its key.

// category is one group of genes in the character panel.
type category struct {
	// id is what the panel remembers the category by, and name and icon
	// what it shows.
	id, name, icon string

	// genes are the keys of the genes in it, in the order they are listed,
	// each with the name it is shown by.
	genes []catalogued
}

// catalogued is a gene the catalogue knows: its key and the name it is shown
// by.
type catalogued struct {
	key, name string
}

// The categories that are not lists of morph genes: the colours of the skin,
// the hair and the eyes, and the genes the catalogue does not know.
const (
	categoryColours = "colours"
	categoryOther   = "other"
)

// categories are the categories of the character panel, in the order it
// lists them. Colours and Other are filled in from the genes themselves.
var categories = []category{
	{id: "head", icon: gui.IconHead, name: "Head", genes: []catalogued{
		{"gene_head_height", "Head Height"},
		{"gene_head_width", "Head Width"},
		{"gene_head_profile", "Profile"},
		{"gene_head_top_height", "Crown Height"},
		{"gene_head_top_width", "Crown Width"},
		{"gene_asian_east", "East Asian Features"},
	}},
	{id: "forehead", icon: gui.IconBrows, name: "Forehead & Brows", genes: []catalogued{
		{"gene_forehead_height", "Forehead Height"},
		{"gene_forehead_width", "Forehead Width"},
		{"gene_forehead_angle", "Forehead Slope"},
		{"gene_forehead_roundness", "Forehead Roundness"},
		{"gene_forehead_brow_height", "Brow Height"},
		{"gene_forehead_brow_width", "Brow Width"},
		{"gene_forehead_brow_forward", "Brow Ridge"},
		{"gene_forehead_brow_curve", "Brow Arch"},
		{"gene_forehead_brow_inner_height", "Inner Brow Height"},
		{"gene_forehead_brow_outer_height", "Outer Brow Height"},
		{"gene_eyebrows_shape", "Eyebrow Shape"},
		{"gene_eyebrows_fullness", "Eyebrow Thickness"},
	}},
	{id: "eyes", icon: gui.IconEyes, name: "Eyes", genes: []catalogued{
		{"gene_eye_size", "Eye Size"},
		{"gene_eye_distance", "Eye Spacing"},
		{"gene_eye_height", "Eye Height"},
		{"gene_eye_angle", "Eye Tilt"},
		{"gene_eye_depth", "Eye Depth"},
		{"gene_eye_shut", "Openness"},
		{"gene_eye_upper_lid_size", "Upper Lid Size"},
		{"gene_eye_fold_shape", "Lid Fold"},
		{"gene_eye_fold_droop", "Lid Droop"},
		{"gene_eye_corner_def", "Corner Definition"},
		{"gene_eye_corner_depth_min", "Inner Corner Depth"},
	}},
	{id: "nose", icon: gui.IconNose, name: "Nose", genes: []catalogued{
		{"gene_nose_size", "Nose Size"},
		{"gene_nose_length", "Length"},
		{"gene_nose_height", "Height"},
		{"gene_nose_forward", "Projection"},
		{"gene_nose_curve", "Bridge Curve"},
		{"gene_nose_hawk", "Hooked Bridge"},
		{"gene_nose_ridge_angle", "Bridge Angle"},
		{"gene_nose_ridge_width", "Bridge Width"},
		{"gene_nose_ridge_def", "Bridge Definition"},
		{"gene_nose_ridge_def_min", "Bridge Softness"},
		{"gene_nose_tip_width", "Tip Width"},
		{"gene_nose_tip_angle", "Tip Angle"},
		{"gene_nose_tip_forward", "Tip Projection"},
		{"gene_nose_nostril_width", "Nostril Width"},
		{"gene_nose_nostril_height", "Nostril Height"},
		{"gene_nose_nostril_angle", "Nostril Flare"},
	}},
	{id: "cheeks", icon: gui.IconCheeks, name: "Cheeks", genes: []catalogued{
		{"gene_cheek_prom", "Cheekbones"},
		{"gene_cheek_height", "Cheek Height"},
		{"gene_cheek_width", "Cheek Width"},
		{"gene_cheek_forward", "Cheek Projection"},
		{"gene_cheek_fat", "Cheek Fullness"},
	}},
	{id: "mouth", icon: gui.IconMouth, name: "Mouth", genes: []catalogued{
		{"gene_mouth_width", "Mouth Width"},
		{"gene_mouth_height", "Mouth Height"},
		{"gene_mouth_forward", "Projection"},
		{"gene_mouth_corner_height", "Corner Height"},
		{"gene_mouth_open", "Lips Parted"},
		{"gene_mouth_upper_lip_size", "Upper Lip Size"},
		{"gene_mouth_upper_lip_full", "Upper Lip Fullness"},
		{"gene_mouth_upper_lip_width", "Upper Lip Width"},
		{"gene_mouth_upper_lip_curve", "Cupid's Bow"},
		{"gene_mouth_upper_lip_def", "Upper Lip Definition"},
		{"gene_mouth_lower_lip_size", "Lower Lip Size"},
		{"gene_mouth_lower_lip_full", "Lower Lip Fullness"},
		{"gene_mouth_lower_lip_width", "Lower Lip Width"},
		{"gene_mouth_lower_lip_def", "Lower Lip Definition"},
		{"gene_mouth_lower_lip_pads", "Lower Lip Pads"},
		{"gene_mouth_philtrum_width", "Philtrum Width"},
		{"gene_mouth_philtrum_def", "Philtrum Depth"},
		{"gene_mouth_philtrum_curve", "Philtrum Curve"},
	}},
	{id: "jaw", icon: gui.IconJaw, name: "Jaw & Chin", genes: []catalogued{
		{"gene_jaw_width", "Jaw Width"},
		{"gene_jaw_height", "Jaw Height"},
		{"gene_jaw_angle", "Jaw Angle"},
		{"gene_jaw_forward", "Jaw Projection"},
		{"gene_jaw_def", "Jaw Definition"},
		{"gene_chin_width", "Chin Width"},
		{"gene_chin_height", "Chin Height"},
		{"gene_chin_forward", "Chin Projection"},
	}},
	{id: "ears", icon: gui.IconEars, name: "Ears", genes: []catalogued{
		{"gene_ear_size", "Ear Size"},
		{"gene_ear_angle", "Ear Angle"},
		{"gene_ear_out", "Protrusion"},
		{"gene_ear_upper_bend", "Upper Fold"},
		{"gene_ear_lower_bend", "Lobe Fold"},
		{"gene_ear_inner_shape", "Inner Shape"},
	}},
	{id: "body", icon: gui.IconBody, name: "Neck & Body", genes: []catalogued{
		{"gene_neck_length", "Neck Length"},
		{"gene_neck_width", "Neck Width"},
		{"gene_bs_body_type", "Build"},
		{"gene_height", "Height"},
	}},
	{id: "ageing", icon: gui.IconAgeing, name: "Ageing & Skin", genes: []catalogued{
		{"gene_age", "Apparent Age"},
		{"gene_old_eyes", "Aged Eyes"},
		{"gene_old_forehead", "Aged Forehead"},
		{"gene_old_nose", "Aged Nose"},
		{"gene_old_mouth", "Aged Mouth"},
		{"gene_old_ears", "Aged Ears"},
		{"gene_crowfeet", "Crow's Feet"},
		{"gene_frown", "Frown Lines"},
		{"gene_surprise", "Forehead Lines"},
		{"gene_complexion", "Complexion"},
		{"gene_stubble", "Stubble"},
		{"gene_face_dacals", "Face Marks"},
	}},
	{id: "hair", icon: gui.IconHair, name: "Hair & Beard", genes: []catalogued{
		{"hairstyles", "Hairstyle"},
		{"beards", "Beard"},
		{"mustaches", "Moustache"},
	}},
	{id: "props", icon: gui.IconProps, name: "Props", genes: []catalogued{
		{"props", "Prop"},
	}},
	{id: categoryColours, icon: gui.IconColours, name: "Colours", genes: []catalogued{
		{"skin_color", "Skin"},
		{"hair_color", "Hair"},
		{"eye_color", "Eyes"},
	}},
	{id: categoryOther, icon: gui.IconCategory, name: "Other"},
}

// hiddenGenes are genes of a DNA the character panel does not list: the
// parts of the head, which have one template each and are not a choice, and
// which the head is drawn with whatever they say.
var hiddenGenes = []string{"eye_accessory", "eye_lashes_accessory", "teeth_accessory"}

// catalogue finds the category and the name of every gene the catalogue
// knows, by its key.
type catalogue map[string]cataloguePlace

// cataloguePlace is where the catalogue puts a gene.
type cataloguePlace struct {
	category int
	name     string
}

// newCatalogue indexes the categories by the keys of their genes.
func newCatalogue() catalogue {
	index := catalogue{}

	for position, category := range categories {
		for _, gene := range category.genes {
			index[gene.key] = cataloguePlace{category: position, name: gene.name}
		}
	}

	return index
}

// place is the category a gene is listed under and the name it is shown by:
// what the catalogue says, or Other and the gene's own key.
func (c catalogue) place(key string) (int, string) {
	if place, ok := c[key]; ok {
		return place.category, place.name
	}

	return otherCategory(), key
}

// otherCategory is the position of Other among the categories.
func otherCategory() int {
	for position, category := range categories {
		if category.id == categoryOther {
			return position
		}
	}

	return len(categories) - 1
}

// friendly turns a key of the game's files into words to show:
// european_hairstyles becomes European Hairstyles. The games' keys are lower
// case words joined by underscores, which says enough of them to read. The
// templates and accessories of genes are shown by their keys instead, as the
// files that are edited by hand name them.
func friendly(key string) string {
	words := strings.FieldsFunc(key, func(r rune) bool { return r == '_' || r == ' ' })

	for index, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[index] = string(runes)
	}

	return strings.Join(words, " ")
}
