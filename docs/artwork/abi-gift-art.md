# Invitation illustrations

Generated with the built-in image generation tool using the original A02/A03/A04 illustrations for the gift outfits and `web/assets/abi-bust.webp` for the animation identity/style reference. The deployable WebP derivatives preserve transparency. These small runtime assets are kept in ordinary Git; source artwork patterns remain covered by Git LFS.

- `abi-size.webp`: Abi in A02’s puff sleeves, braided belt and sunglasses, holding a small Nordic dress on a wooden hanger.
- `abi-envelopes.webp`: Abi in A03’s dark fur cape and layered dress, holding cream gift envelopes with gold seals.
- `abi-favorites.webp`: Abi in A04’s embroidered long sleeves and woven sash with Toothless and Teletubbies plush toys.
- `viking-map.webp`: folded parchment, winding route, location pin and Viking longhouse.
- `abi-smile-1.webp`, `abi-smile-2.webp`: progressively smiling portrait frames with gently shifted curls, based on the existing bust.

The portrait blends the original and two generated frames in a fixed-size canvas as the page scrolls. This is a three-keyframe illustration animation, not a continuous video. Scrolling upward reverses it. Reduced motion displays the original image without loading extra frames. Failed frame downloads also retain the original. The map icon and gift illustrations are decorative; the adjacent text conveys their meaning.
