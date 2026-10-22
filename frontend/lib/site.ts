// Site-wide facts. Only confirmed values are filled in (brief DQ-01): every
// contact field is empty and its UI hides itself until a real value exists.
// From Stage 6 these come from site settings in the API, with this file as
// the fallback snapshot.

export type SiteContact = {
  phone: string;
  email: string;
  whatsapp: string;
  address: string;
  officeHours: string;
};

export type SiteConfig = {
  name: string;
  location: string;
  contact: SiteContact;
  /** Registration or accreditation line for the footer. Empty until verified (DQ-04, NN-01). */
  registration: string;
};

export const site: SiteConfig = {
  name: "Dakota Training College",
  location: "Makueni, Kenya",
  contact: { phone: "", email: "", whatsapp: "", address: "", officeHours: "" },
  registration: "",
};
