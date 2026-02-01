/**
 * color-hash.js
 * https://github.com/zenozeng/color-hash.js
 *
 * Create a deterministic color from a string.
 */
var ColorHash = function (options) {
    options = options || {};

    var HSL_RANGE = options.hsl_range || [[30, 330], [30, 75], [60, 90]];
    var HUE_RANGE = options.hue_range || [30, 330];
    var SATURATION_RANGE = options.saturation_range || [30, 75];
    var LIGHTNESS_RANGE = options.lightness_range || [60, 90];

    var djb2 = function(str) {
        var hash = 5381;
        for (var i = 0; i < str.length; i++) {
            hash = ((hash << 5) + hash) + str.charCodeAt(i);
        }
        return hash;
    }

    this.hsl = function(str) {
        var hash = djb2(str);
        var h = (hash % (HUE_RANGE[1] - HUE_RANGE[0])) + HUE_RANGE[0];
        var s = (hash % (SATURATION_RANGE[1] - SATURATION_RANGE[0])) + SATURATION_RANGE[0];
        var l = (hash % (LIGHTNESS_RANGE[1] - LIGHTNESS_RANGE[0])) + LIGHTNESS_RANGE[0];
        return [h, s, l];
    }

    this.rgb = function(str) {
        var hsl = this.hsl(str);
        var h = hsl[0] / 360;
        var s = hsl[1] / 100;
        var l = hsl[2] / 100;
        var t2, t3, val;

        if (s == 0) {
            val = l * 255;
            return [val, val, val];
        }

        if (l < 0.5) {
            t2 = l * (1 + s);
        } else {
            t2 = l + s - l * s;
        }

        var t1 = 2 * l - t2;

        var rgb = [0, 0, 0];
        for (var i = 0; i < 3; i++) {
            t3 = h + 1/3 * - (i - 1);
            if (t3 < 0) t3++;
            if (t3 > 1) t3--;

            if (6 * t3 < 1)
                val = t1 + (t2 - t1) * 6 * t3;
            else if (2 * t3 < 1)
                val = t2;
            else if (3 * t3 < 2)
                val = t1 + (t2 - t1) * (2/3 - t3) * 6;
            else
                val = t1;

            rgb[i] = val * 255;
        }

        return rgb;
    }

    this.hex = function(str) {
        var rgb = this.rgb(str);
        var r = Math.round(rgb[0]).toString(16);
        var g = Math.round(rgb[1]).toString(16);
        var b = Math.round(rgb[2]).toString(16);
        return "#" + r + g + b;
    }
}
