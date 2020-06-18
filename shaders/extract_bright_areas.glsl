#version 330 core
in vec4  vColor;
in vec2  vTexCoords;

out vec4 fragColor;

uniform sampler2D uTexture;
uniform vec4      uTexBounds;
void main() {
	vec2 uv = gl_FragCoord.xy / uTexBounds.zw;
	vec2  PixelOffset = 1.0 / uTexBounds.zw;

	vec4 c = texture(uTexture, uv);
	float brightness = (c.r + c.g + c.b + c.a) / 4.0;

	fragColor = c;
	if (brightness < 0.4) {
		fragColor = c * brightness;
	} else {
		fragColor = c * (1 + (1 - brightness));
	}
}
