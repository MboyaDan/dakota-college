package application

// Counties are Kenya's 47 counties (Constitution of Kenya 2010, First
// Schedule), for the "county of residence" choice, plus one option for
// applicants who live outside Kenya.
var Counties = []string{
	"Baringo", "Bomet", "Bungoma", "Busia", "Elgeyo-Marakwet", "Embu", "Garissa", "Homa Bay", "Isiolo",
	"Kajiado", "Kakamega", "Kericho", "Kiambu", "Kilifi", "Kirinyaga", "Kisii", "Kisumu", "Kitui", "Kwale",
	"Laikipia", "Lamu", "Machakos", "Makueni", "Mandera", "Marsabit", "Meru", "Migori", "Mombasa",
	"Murang'a", "Nairobi", "Nakuru", "Nandi", "Narok", "Nyamira", "Nyandarua", "Nyeri", "Samburu", "Siaya",
	"Taita-Taveta", "Tana River", "Tharaka-Nithi", "Trans Nzoia", "Turkana", "Uasin Gishu", "Vihiga",
	"Wajir", "West Pokot", OutsideKenya,
}

const OutsideKenya = "Outside Kenya"

// KCSEGrades are the KCSE grades, best first, for mean and subject grades.
var KCSEGrades = []string{"A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D+", "D", "D-", "E"}
