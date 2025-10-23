export const toScreenCoords = (
	x: number,
	y: number,
	elementWidth: number,
	elementHeight: number,
	width: number,
	height: number
) => {
	// Calculate the displayed size with object-contain
	const imageAspect = width / height;
	const elementAspect = elementWidth / elementHeight;

	let displayWidth, displayHeight, offsetX, offsetY;

	if (elementAspect > imageAspect) {
		// Element is wider - letterbox on sides
		displayHeight = elementHeight;
		displayWidth = elementHeight * imageAspect;
		offsetX = (elementWidth - displayWidth) / 2;
		offsetY = 0;
	} else {
		// Element is taller - letterbox on top/bottom
		displayWidth = elementWidth;
		displayHeight = elementWidth / imageAspect;
		offsetX = 0;
		offsetY = (elementHeight - displayHeight) / 2;
	}

	// Adjust for letterboxing offset
	const adjustedX = x - offsetX;
	const adjustedY = y - offsetY;

	// Scale to actual screen coordinates
	const scaleX = width / displayWidth;
	const scaleY = height / displayHeight;

	return {
		x: Math.round(adjustedX * scaleX),
		y: Math.round(adjustedY * scaleY)
	};
};
