import type { components } from "@/lib/api/schema";

type ImageFile = components["schemas"]["ImageFile"];

/**
 * An uploaded image with its resized widths (D-59), so phones download the
 * small one. Lazy by default; the API's alt text is required by the editor.
 */
export function ContentImage({
  image,
  alt,
  sizes,
  className,
  priority = false,
}: {
  image: ImageFile;
  alt: string;
  sizes: string;
  className?: string;
  priority?: boolean;
}) {
  const srcSet = image.variants.length
    ? image.variants.map((v) => `${v.url} ${v.width}w`).join(", ")
    : undefined;
  return (
    // Uploaded files are already resized and stripped of metadata by the API.
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={image.url}
      srcSet={srcSet}
      sizes={sizes}
      alt={alt}
      width={image.width || undefined}
      height={image.height || undefined}
      loading={priority ? "eager" : "lazy"}
      fetchPriority={priority ? "high" : undefined}
      decoding="async"
      className={className}
    />
  );
}
