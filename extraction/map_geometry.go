package extraction

import (
	"math"
)

// cmToInt16 converts centimeters to int16 at 10cm scale (for Z/altitude).
func cmToInt16(cm float64) int16 {
	v := math.Round(cm / 10.0)
	if v > 32767 {
		v = 32767
	}
	if v < -32768 {
		v = -32768
	}
	return int16(v)
}

// cmToInt32 converts centimeters to int32 at 1cm scale (for object X/Y positions).
// 1cm precision so walls and structures connect precisely on the map.
func cmToInt32(cm float64) int32 {
	return int32(math.Round(cm))
}

// cmToUint16 converts centimeters to uint16 at 1cm scale (for dimension overrides).
// Max 65535cm = 655.35m, plenty for any structure dimension.
func cmToUint16(cm float64) uint16 {
	v := math.Round(cm)
	if v < 0 {
		return 0
	}
	if v > 65535 {
		return 65535
	}
	return uint16(v)
}

// metersToInt16 converts world meters to int16 at 10cm scale.
func metersToInt16(m float64) int16 {
	return cmToInt16(m * 100)
}

// metersToInt32 converts world meters to int32 at 10cm scale (for polyline points).
// Polylines stay at 10cm — they don't need wall-precision and int8 deltas would
// cover only ±1.27m at 1cm, causing excessive 16-bit fallbacks.
func metersToInt32(m float64) int32 {
	return int32(math.Round(m * 10))
}

// float16 converts a float64 to IEEE 754 half-precision (float16) bits as uint16.
// Used for compact tangent encoding in polylines. Precision is ~3 decimal digits,
// sufficient for tangent direction vectors in world meters.
func float16(f float64) uint16 {
	x := math.Float32bits(float32(f))
	sign := uint16((x >> 31) & 1)
	exp := int((x >> 23) & 0xFF)
	mant := uint32(x & 0x7FFFFF)

	if exp == 0xFF {
		// Inf or NaN
		if mant != 0 {
			return (sign << 15) | 0x7C00 | 1 // NaN
		}
		return (sign << 15) | 0x7C00 // Inf
	}

	if exp == 0 && mant == 0 {
		return sign << 15 // zero
	}

	// Convert from float32 exponent (bias 127) to float16 exponent (bias 15)
	newExp := exp - 127 + 15

	if newExp >= 0x1F {
		// Overflow -> Inf
		return (sign << 15) | 0x7C00
	}

	if newExp <= 0 {
		// Subnormal or zero
		if newExp < -10 {
			return sign << 15 // too small -> zero
		}
		// Subnormal: shift mantissa, include implicit leading 1
		mant |= 0x800000 // add implicit leading 1
		shift := uint(14 - newExp)
		halfMant := mant >> shift
		// Round to nearest, ties to even
		if shift > 0 && mant&(1<<(shift-1)) != 0 {
			halfMant++
		}
		return (sign << 15) | uint16(halfMant)
	}

	// Normal: shift mantissa from 23 to 10 bits
	halfMant := mant >> 13
	// Round to nearest, ties to even
	if mant&0x1000 != 0 {
		halfMant++
		if halfMant&0x400 != 0 {
			// Mantissa overflowed -> increment exponent
			halfMant = 0
			newExp++
			if newExp >= 0x1F {
				return (sign << 15) | 0x7C00
			}
		}
	}
	return (sign << 15) | (uint16(newExp) << 10) | uint16(halfMant&0x3FF)
}

// simplifyPoints reduces a polyline to its start and end point. When it does
// reduce it returns a freshly allocated slice — slicing the input and appending
// to it would write back into the caller's array, corrupting the source
// polyline. Inputs of two points or fewer are already minimal and are returned
// as-is, so callers must not mutate the result.
// Closed loops (start == end) keep their midpoint so they do not collapse to
// a zero-length segment.
func simplifyPoints(pts [][2]float64) [][2]float64 {
	if len(pts) <= 2 {
		return pts
	}
	first := pts[0]
	last := pts[len(pts)-1]
	if first[0] == last[0] && first[1] == last[1] {
		return [][2]float64{first, pts[len(pts)/2], last}
	}
	return [][2]float64{first, last}
}

// snapExtendM is how far a belt/pipe endpoint is pushed toward the building it
// connects to. Belts store their own transform position, which sits a few metres
// outside the building footprint and leaves a visible gap on the map. The
// building rectangle is drawn on top, so it hides the overshoot.
const snapExtendM = 5.0

// minStubM is the shortest stub worth emitting. Below this the endpoint already
// sits essentially on the building centre and a stub would be degenerate.
const minStubM = 0.5

// buildingStub returns the point snapExtendM metres from p toward the nearest
// building centre, reporting false when no building is in range or the endpoint
// already coincides with one.
func buildingStub(p [2]float64, findNearestBuilding func(x, y float64) (float64, float64, bool)) ([2]float64, bool) {
	bx, by, ok := findNearestBuilding(p[0], p[1])
	if !ok {
		return p, false
	}
	dx, dy := bx-p[0], by-p[1]
	dist := math.Hypot(dx, dy)
	if dist < minStubM {
		return p, false
	}
	move := math.Min(snapExtendM, dist)
	return [2]float64{p[0] + dx/dist*move, p[1] + dy/dist*move}, true
}

// axialStub extends endpoint p outward along the polyline's own direction, far
// enough to reach into the building it connects to.
//
// The extension direction is the polyline's end tangent, NOT the bearing to the
// building centre. A belt enters a building along its own axis, so extending
// toward the centre would grow a visible barb at an arbitrary angle — including
// straight backwards when the nearest grid entry happens to be a splitter or
// lift sitting behind the endpoint.
//
// outTan is the outward tangent at p (negated leave tangent at the start of a
// polyline, arrive tangent at the end); fallback is the chord to the neighbouring
// control point, used when the save stored a zero tangent. Reports false when no
// building is in range, so belts ending in mid-air are left alone.
func axialStub(
	p, neighbour [2]float64,
	outTan [2]float64,
	findNearestBuilding func(x, y float64) (float64, float64, bool),
) ([2]float64, bool) {
	bx, by, ok := findNearestBuilding(p[0], p[1])
	if !ok {
		return p, false
	}
	dist := math.Hypot(bx-p[0], by-p[1])
	if dist < minStubM {
		return p, false
	}
	dirX, dirY := outTan[0], outTan[1]
	if l := math.Hypot(dirX, dirY); l > 1e-6 {
		dirX, dirY = dirX/l, dirY/l
	} else {
		dirX, dirY = p[0]-neighbour[0], p[1]-neighbour[1]
		l = math.Hypot(dirX, dirY)
		if l <= 1e-6 {
			return p, false
		}
		dirX, dirY = dirX/l, dirY/l
	}
	move := math.Min(snapExtendM, dist)
	return [2]float64{p[0] + dirX*move, p[1] + dirY*move}, true
}

// snapTangentEnds extends a tangent-mode polyline into the buildings at either
// end by adding a short straight stub control point, collinear with the
// polyline's own end direction (see axialStub).
//
// The stub is a *new* control point rather than a displacement of the existing
// endpoint: an endpoint's position feeds the Bezier control point of its span
// (CP1 = P0 + LeaveTan/3), so moving it would bend a curve that happens to end
// at a building. Adding a point leaves the original curve untouched.
//
// Both tangents bounding a stub span are set to zero, which places the Bezier
// control points exactly on the span endpoints (CP1 = P0, CP2 = P1) and so
// reproduces the chord exactly. Deriving the tangents from the stub vector
// instead would inherit the 10cm position quantisation and bow the stub by a few
// centimetres, occasionally tipping it past the frontend's flatness tolerance.
//
// The slots written here are exactly the ones that are never read otherwise: the
// arrive tangent of the first control point and the leave tangent of the last.
// The input slices are not modified.
func snapTangentEnds(
	pts [][2]float64,
	tans [][4]float64,
	findNearestBuilding func(x, y float64) (float64, float64, bool),
	isChainJoint func(p [2]float64) bool,
) ([][2]float64, [][4]float64) {
	lastIdx := len(pts) - 1
	// Outward directions: away from the polyline at each end. The leave tangent
	// points along travel, so negate it at the start; the arrive tangent already
	// points outward at the end. Ends that meet another piece of the same run are
	// left alone — there is no gap there to fill.
	var startStub, endStub [2]float64
	var hasStart, hasEnd bool
	if !isChainJoint(pts[0]) {
		startStub, hasStart = axialStub(pts[0], pts[1],
			[2]float64{-tans[0][0], -tans[0][1]}, findNearestBuilding)
	}
	if !isChainJoint(pts[lastIdx]) {
		endStub, hasEnd = axialStub(pts[lastIdx], pts[lastIdx-1],
			[2]float64{tans[lastIdx][2], tans[lastIdx][3]}, findNearestBuilding)
	}
	if !hasStart && !hasEnd {
		return pts, tans
	}

	total := len(pts)
	if hasStart {
		total++
	}
	if hasEnd {
		total++
	}
	np := make([][2]float64, 0, total)
	nt := make([][4]float64, 0, total)

	if hasStart {
		// Stub runs startStub -> original first point.
		np = append(np, startStub)
		nt = append(nt, [4]float64{})
	}
	np = append(np, pts...)
	nt = append(nt, tans...)
	if hasStart {
		// The original first point now terminates the stub span.
		nt[1][2] = 0
		nt[1][3] = 0
	}
	if hasEnd {
		// Stub runs original last point -> endStub.
		li := len(np) - 1
		nt[li][0] = 0
		nt[li][1] = 0
		np = append(np, endStub)
		nt = append(nt, [4]float64{})
	}
	return np, nt
}

// quaternionToYaw10 converts a quaternion to yaw in degrees*10 (0-3599).
// Uses the projection approach: rotates the local X axis (1,0,0) by the full
// quaternion and projects to the XY plane. This correctly handles 3D-rotated
// objects (signs, pillars) where the standard UE yaw extraction formula gives
// wrong results. Falls back to the standard formula when the local X axis is
// nearly vertical (XY projection near zero), since the yaw is undefined for
// vertical objects and the standard formula is a reasonable default.
func quaternionToYaw10(qx, qy, qz, qw float32) int16 {
	rx, ry, _ := rotate3DByQuaternion(1, 0, 0, float64(qx), float64(qy), float64(qz), float64(qw))
	xyLen := math.Sqrt(rx*rx + ry*ry)
	if xyLen < 0.01 {
		// Local X axis is vertical — fall back to standard formula
		yawRad := math.Atan2(2.0*(float64(qw)*float64(qz)+float64(qx)*float64(qy)),
			1.0-2.0*(float64(qy)*float64(qy)+float64(qz)*float64(qz)))
		yawDeg := yawRad * 180.0 / math.Pi
		if yawDeg < 0 {
			yawDeg += 360
		}
		return int16(math.Round(yawDeg * 10))
	}
	yawRad := math.Atan2(ry, rx)
	yawDeg := yawRad * 180.0 / math.Pi
	if yawDeg < 0 {
		yawDeg += 360
	}
	return int16(math.Round(yawDeg * 10))
}

// quaternionToBeamYaw10 computes the effective yaw for a beam by rotating the
// local X axis (1,0,0) by the full quaternion and projecting to the XY plane.
// This correctly handles beams with non-yaw quaternions (e.g. tilted or flipped beams)
// where the standard yaw extraction gives the wrong direction.
// Returns (yaw10, dirX, dirY) where (dirX, dirY) is the XY-projected direction
// of the beam's local X axis (used to offset the beam center, since beams
// extend from 0 to length in local X, not -length/2 to +length/2).
func quaternionToBeamYaw10(qx, qy, qz, qw float32) (int16, float64, float64) {
	// Rotate (1,0,0) by the quaternion
	rx, ry, _ := rotate3DByQuaternion(1, 0, 0, float64(qx), float64(qy), float64(qz), float64(qw))
	// If the beam is nearly vertical (XY projection near zero), don't apply length override
	xyLen := math.Sqrt(rx*rx + ry*ry)
	if xyLen < 0.01 {
		return 0, 0, 0
	}
	yawRad := math.Atan2(ry, rx)
	yawDeg := yawRad * 180.0 / math.Pi
	if yawDeg < 0 {
		yawDeg += 360
	}
	return int16(math.Round(yawDeg * 10)), rx, ry
}

// rotateByQuaternion rotates a 2D point (x, y, 0) by a quaternion, returning the XY projection.
func rotateByQuaternion(x, y, qx, qy, qz, qw float64) (float64, float64) {
	rx, ry, _ := rotate3DByQuaternion(x, y, 0, qx, qy, qz, qw)
	return rx, ry
}

// rotate3DByQuaternion rotates a 3D vector by a quaternion, returning the full 3D result.
func rotate3DByQuaternion(x, y, z, qx, qy, qz, qw float64) (float64, float64, float64) {
	// v' = q * v * q^-1
	// t = 2 * cross(q.xyz, v)
	tx := 2 * (qy*z - qz*y)
	ty := 2 * (qz*x - qx*z)
	tz := 2 * (qx*y - qy*x)
	// v' = v + qw * t + cross(q.xyz, t)
	rx := x + qw*tx + (qy*tz - qz*ty)
	ry := y + qw*ty + (qz*tx - qx*tz)
	rz := z + qw*tz + (qx*ty - qy*tx)
	return rx, ry, rz
}
