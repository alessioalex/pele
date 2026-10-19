package js

import "math"

// The Math functions below are ports of V8's src/base/ieee754.cc (fdlibm), which Node 22
// uses for Math.sin, Math.pow, etc. (V8_USE_LIBM_TRIG_FUNCTIONS is off there), so they
// return the same bits as V8 rather than Go's slightly different results.
//
// Products that feed an addition or subtraction are wrapped in float64() so that no
// architecture fuses them into an FMA: V8's x64 build rounds every product.

func hiWord(x float64) int32  { return int32(math.Float64bits(x) >> 32) }
func loWord(x float64) uint32 { return uint32(math.Float64bits(x)) }

func words(x float64) (int32, uint32) {
	b := math.Float64bits(x)
	return int32(b >> 32), uint32(b)
}

func fromWords(hi int32, lo uint32) float64 {
	return math.Float64frombits(uint64(uint32(hi))<<32 | uint64(lo))
}

func setHiWord(x float64, hi int32) float64  { return fromWords(hi, loWord(x)) }
func setLoWord(x float64, lo uint32) float64 { return fromWords(hiWord(x), lo) }

// scalbn is fdlibm's s_scalbn.c: x*2^n, correctly rounded.
func scalbn(x float64, n int32) float64 {
	const (
		two54  float64 = 1.80143985094819840000e+16
		twom54 float64 = 5.55111512312578270212e-17
	)
	hx, lx := words(x)
	k := (hx & 0x7ff00000) >> 20
	if k == 0 { // 0 or subnormal x
		if lx|uint32(hx&0x7fffffff) == 0 {
			return x
		}
		x *= two54
		hx = hiWord(x)
		k = ((hx & 0x7ff00000) >> 20) - 54
		if n < -50000 {
			return math.Copysign(0, x)
		}
	}
	if k == 0x7ff {
		return x + x
	}
	k += n
	if k > 0x7fe {
		return math.Copysign(math.Inf(1), x)
	}
	hm := int32(uint32(hx) & 0x800fffff) // sign and high mantissa
	if k > 0 {
		return setHiWord(x, hm|(k<<20))
	}
	if k <= -54 {
		if n > 50000 {
			return math.Copysign(math.Inf(1), x)
		}
		return math.Copysign(0, x)
	}
	k += 54
	return setHiWord(x, hm|(k<<20)) * twom54
}

func Sin(x float64) float64 {
	ix := hiWord(x) & 0x7fffffff
	if ix <= 0x3fe921fb {
		return kernelSin(x, 0, 0)
	}
	if ix >= 0x7ff00000 {
		return x - x
	}
	n, y0, y1 := remPio2(x)
	switch n & 3 {
	case 0:
		return kernelSin(y0, y1, 1)
	case 1:
		return kernelCos(y0, y1)
	case 2:
		return -kernelSin(y0, y1, 1)
	default:
		return -kernelCos(y0, y1)
	}
}

func Cos(x float64) float64 {
	ix := hiWord(x) & 0x7fffffff
	if ix <= 0x3fe921fb {
		return kernelCos(x, 0)
	}
	if ix >= 0x7ff00000 {
		return x - x
	}
	n, y0, y1 := remPio2(x)
	switch n & 3 {
	case 0:
		return kernelCos(y0, y1)
	case 1:
		return -kernelSin(y0, y1, 1)
	case 2:
		return -kernelCos(y0, y1)
	default:
		return kernelSin(y0, y1, 1)
	}
}

func Tan(x float64) float64 {
	ix := hiWord(x) & 0x7fffffff
	if ix <= 0x3fe921fb {
		return kernelTan(x, 0, 1)
	}
	if ix >= 0x7ff00000 {
		return x - x
	}
	n, y0, y1 := remPio2(x)
	return kernelTan(y0, y1, 1-((n&1)<<1))
}

// Table of 2/pi, 396 hex digits.
var twoOverPi = [...]int32{
	0xA2F983, 0x6E4E44, 0x1529FC, 0x2757D1, 0xF534DD, 0xC0DB62, 0x95993C,
	0x439041, 0xFE5163, 0xABDEBB, 0xC561B7, 0x246E3A, 0x424DD2, 0xE00649,
	0x2EEA09, 0xD1921C, 0xFE1DEB, 0x1CB129, 0xA73EE8, 0x8235F5, 0x2EBB44,
	0x84E99C, 0x7026B4, 0x5F7E41, 0x3991D6, 0x398353, 0x39F49C, 0x845F8B,
	0xBDF928, 0x3B1FF8, 0x97FFDE, 0x05980F, 0xEF2F11, 0x8B5A0A, 0x6D1F6D,
	0x367ECF, 0x27CB09, 0xB74F46, 0x3F669E, 0x5FEA2D, 0x7527BA, 0xC7EBE5,
	0xF17B3D, 0x0739F7, 0x8A5292, 0xEA6BFB, 0x5FB11F, 0x8D5D08, 0x560330,
	0x46FC7B, 0x6BABF0, 0xCFBC20, 0x9AF436, 0x1DA9E3, 0x91615E, 0xE61B08,
	0x659985, 0x5F14A0, 0x68408D, 0xFFD880, 0x4D7327, 0x310606, 0x1556CA,
	0x73A8C9, 0x60E27B, 0xC08C6B,
}

var npio2hw = [...]int32{
	0x3FF921FB, 0x400921FB, 0x4012D97C, 0x401921FB, 0x401F6A7A, 0x4022D97C,
	0x4025FDBB, 0x402921FB, 0x402C463A, 0x402F6A7A, 0x4031475C, 0x4032D97C,
	0x40346B9C, 0x4035FDBB, 0x40378FDB, 0x403921FB, 0x403AB41B, 0x403C463A,
	0x403DD85A, 0x403F6A7A, 0x40407E4C, 0x4041475C, 0x4042106C, 0x4042D97C,
	0x4043A28C, 0x40446B9C, 0x404534AC, 0x4045FDBB, 0x4046C6CB, 0x40478FDB,
	0x404858EB, 0x404921FB,
}

// remPio2 is __ieee754_rem_pio2: it returns n and y0+y1 = x - n*pi/2, |y0+y1| <= pi/4.
func remPio2(x float64) (int32, float64, float64) {
	const (
		zero    float64 = 0.00000000000000000000e+00
		half    float64 = 5.00000000000000000000e-01
		two24   float64 = 1.67772160000000000000e+07
		invpio2 float64 = 6.36619772367581382433e-01
		pio2_1  float64 = 1.57079632673412561417e+00
		pio2_1t float64 = 6.07710050650619224932e-11
		pio2_2  float64 = 6.07710050630396597660e-11
		pio2_2t float64 = 2.02226624879595063154e-21
		pio2_3  float64 = 2.02226624871116645580e-21
		pio2_3t float64 = 8.47842766036889956997e-32
	)
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	if ix <= 0x3fe921fb {
		return 0, x, 0
	}
	if ix < 0x4002d97c { // |x| < 3pi/4, n = +-1
		if hx > 0 {
			z := x - pio2_1
			if ix != 0x3ff921fb {
				y0 := z - pio2_1t
				return 1, y0, (z - y0) - pio2_1t
			}
			z -= pio2_2
			y0 := z - pio2_2t
			return 1, y0, (z - y0) - pio2_2t
		}
		z := x + pio2_1
		if ix != 0x3ff921fb {
			y0 := z + pio2_1t
			return -1, y0, (z - y0) + pio2_1t
		}
		z += pio2_2
		y0 := z + pio2_2t
		return -1, y0, (z - y0) + pio2_2t
	}
	if ix <= 0x413921fb { // |x| ~<= 2^19*(pi/2)
		t := math.Abs(x)
		n := int32(float64(t*invpio2) + half)
		fn := float64(n)
		r := t - float64(fn*pio2_1)
		w := float64(fn * pio2_1t)
		var y0 float64
		if n < 32 && ix != npio2hw[n-1] {
			y0 = r - w
		} else {
			j := ix >> 20
			y0 = r - w
			i := j - ((hiWord(y0) >> 20) & 0x7ff)
			if i > 16 { // 2nd iteration, good to 118 bits
				t = r
				w = float64(fn * pio2_2)
				r = t - w
				w = float64(fn*pio2_2t) - ((t - r) - w)
				y0 = r - w
				i = j - ((hiWord(y0) >> 20) & 0x7ff)
				if i > 49 { // 3rd iteration, 151 bits
					t = r
					w = float64(fn * pio2_3)
					r = t - w
					w = float64(fn*pio2_3t) - ((t - r) - w)
					y0 = r - w
				}
			}
		}
		y1 := (r - y0) - w
		if hx < 0 {
			return -n, -y0, -y1
		}
		return n, y0, y1
	}
	if ix >= 0x7ff00000 {
		y := x - x
		return 0, y, y
	}
	// z = scalbn(|x|, ilogb(x)-23), split into 24-bit chunks
	e0 := (ix >> 20) - 1046
	z := fromWords(ix-(e0<<20), loWord(x))
	var tx [3]float64
	for i := 0; i < 2; i++ {
		tx[i] = float64(int32(z))
		z = float64((z - tx[i]) * two24)
	}
	tx[2] = z
	nx := int32(3)
	for tx[nx-1] == zero {
		nx--
	}
	n, y0, y1 := kernelRemPio2(&tx, e0, nx)
	if hx < 0 {
		return -n, -y0, -y1
	}
	return n, y0, y1
}

// pi/2 cut into 24-bit chunks.
var pio2Chunks = [...]float64{
	1.57079625129699707031e+00,
	7.54978941586159635335e-08,
	5.39030252995776476554e-15,
	3.28200341580791294123e-22,
	1.27065575308067607349e-29,
	1.22933308981111328932e-36,
	2.73370053816464559624e-44,
	2.16741683877804819444e-51,
}

// kernelRemPio2 is __kernel_rem_pio2 with prec = 2, as V8 calls it.
func kernelRemPio2(x *[3]float64, e0, nx int32) (int32, float64, float64) {
	const (
		jk     int32   = 4 // init_jk[2]
		jp             = jk
		zero   float64 = 0.0
		one    float64 = 1.0
		two24  float64 = 1.67772160000000000000e+07
		twon24 float64 = 5.96046447753906250000e-08
	)
	var (
		iq       [20]int32
		f, fq, q [20]float64
	)
	jx := nx - 1
	jv := (e0 - 3) / 24
	if jv < 0 {
		jv = 0
	}
	q0 := e0 - 24*(jv+1)

	// f[0..jx+jk] = ipio2[jv-jx..jv+jk]
	j := jv - jx
	m := jx + jk
	for i := int32(0); i <= m; i, j = i+1, j+1 {
		if j < 0 {
			f[i] = zero
		} else {
			f[i] = float64(twoOverPi[j])
		}
	}
	for i := int32(0); i <= jk; i++ {
		fw := 0.0
		for j := int32(0); j <= jx; j++ {
			fw += float64(x[j] * f[jx+i-j])
		}
		q[i] = fw
	}

	jz := jk
	var z float64
	var n, ih int32
	for { // recompute
		// distill q[] into iq[] reversingly
		z = q[jz]
		for i, j := int32(0), jz; j > 0; i, j = i+1, j-1 {
			fw := float64(int32(twon24 * z))
			iq[i] = int32(z - float64(two24*fw))
			z = q[j-1] + fw
		}

		z = scalbn(z, q0)
		z -= float64(8.0 * math.Floor(z*0.125)) // trim off integer >= 8
		n = int32(z)
		z -= float64(n)
		ih = 0
		if q0 > 0 { // need iq[jz-1] to determine n
			i := iq[jz-1] >> (24 - q0)
			n += i
			iq[jz-1] -= i << (24 - q0)
			ih = iq[jz-1] >> (23 - q0)
		} else if q0 == 0 {
			ih = iq[jz-1] >> 23
		} else if z >= 0.5 {
			ih = 2
		}

		if ih > 0 { // q > 0.5
			n++
			carry := int32(0)
			for i := int32(0); i < jz; i++ { // compute 1-q
				j := iq[i]
				if carry == 0 {
					if j != 0 {
						carry = 1
						iq[i] = 0x1000000 - j
					}
				} else {
					iq[i] = 0xffffff - j
				}
			}
			switch q0 {
			case 1:
				iq[jz-1] &= 0x7fffff
			case 2:
				iq[jz-1] &= 0x3fffff
			}
			if ih == 2 {
				z = one - z
				if carry != 0 {
					z -= scalbn(one, q0)
				}
			}
		}

		if z == zero {
			j := int32(0)
			for i := jz - 1; i >= jk; i-- {
				j |= iq[i]
			}
			if j == 0 { // need recomputation
				k := int32(1)
				for jk >= k && iq[jk-k] == 0 {
					k++
				}
				for i := jz + 1; i <= jz+k; i++ { // add q[jz+1] to q[jz+k]
					f[jx+i] = float64(twoOverPi[jv+i])
					fw := 0.0
					for j := int32(0); j <= jx; j++ {
						fw += float64(x[j] * f[jx+i-j])
					}
					q[i] = fw
				}
				jz += k
				continue
			}
		}
		break
	}

	// chop off zero terms
	if z == 0.0 {
		jz--
		q0 -= 24
		for iq[jz] == 0 {
			jz--
			q0 -= 24
		}
	} else { // break z into 24-bit if necessary
		z = scalbn(z, -q0)
		if z >= two24 {
			fw := float64(int32(twon24 * z))
			iq[jz] = int32(z - float64(two24*fw))
			jz++
			q0 += 24
			iq[jz] = int32(fw)
		} else {
			iq[jz] = int32(z)
		}
	}

	// convert integer "bit" chunk to floating-point value
	fw := scalbn(one, q0)
	for i := jz; i >= 0; i-- {
		q[i] = fw * float64(iq[i])
		fw *= twon24
	}

	// compute PIo2[0,...,jp]*q[jz,...,0]
	for i := jz; i >= 0; i-- {
		fw = 0.0
		for k := int32(0); k <= jp && k <= jz-i; k++ {
			fw += float64(pio2Chunks[k] * q[i+k])
		}
		fq[jz-i] = fw
	}

	// compress fq[] into y[]
	fw = 0.0
	for i := jz; i >= 0; i-- {
		fw += fq[i]
	}
	y0 := fw
	fw = fq[0] - fw
	for i := int32(1); i <= jz; i++ {
		fw += fq[i]
	}
	y1 := fw
	if ih != 0 {
		y0, y1 = -y0, -y1
	}
	return n & 7, y0, y1
}

// kernelSin is __kernel_sin on [-pi/4, pi/4]; y is the tail of x, iy = 0 if y is 0.
func kernelSin(x, y float64, iy int) float64 {
	const (
		half float64 = 5.00000000000000000000e-01
		S1   float64 = -1.66666666666666324348e-01
		S2   float64 = 8.33333333332248946124e-03
		S3   float64 = -1.98412698298579493134e-04
		S4   float64 = 2.75573137070700676789e-06
		S5   float64 = -2.50507602534068634195e-08
		S6   float64 = 1.58969099521155010221e-10
	)
	ix := hiWord(x) & 0x7fffffff
	if ix < 0x3e400000 { // |x| < 2**-27
		return x
	}
	z := x * x
	v := z * x
	r := S2 + float64(z*(S3+float64(z*(S4+float64(z*(S5+float64(z*S6)))))))
	if iy == 0 {
		return x + float64(v*(S1+float64(z*r)))
	}
	return x - ((float64(z*(float64(half*y)-float64(v*r))) - y) - float64(v*S1))
}

// kernelCos is __kernel_cos on [-pi/4, pi/4]; y is the tail of x.
func kernelCos(x, y float64) float64 {
	const (
		one float64 = 1.00000000000000000000e+00
		C1  float64 = 4.16666666666666019037e-02
		C2  float64 = -1.38888888888741095749e-03
		C3  float64 = 2.48015872894767294178e-05
		C4  float64 = -2.75573143513906633035e-07
		C5  float64 = 2.08757232129817482790e-09
		C6  float64 = -1.13596475577881948265e-11
	)
	ix := hiWord(x) & 0x7fffffff
	if ix < 0x3e400000 { // |x| < 2**-27
		return one
	}
	z := x * x
	r := z * (C1 + float64(z*(C2+float64(z*(C3+float64(z*(C4+float64(z*(C5+float64(z*C6))))))))))
	if ix < 0x3fd33333 { // |x| < 0.3
		return one - (float64(0.5*z) - (float64(z*r) - float64(x*y)))
	}
	var qx float64
	if ix > 0x3fe90000 { // x > 0.78125
		qx = 0.28125
	} else {
		qx = fromWords(ix-0x00200000, 0) // x/4
	}
	iz := float64(0.5*z) - qx
	a := one - qx
	return a - (iz - (float64(z*r) - float64(x*y)))
}

var tanT = [...]float64{
	3.33333333333334091986e-01,
	1.33333333333201242699e-01,
	5.39682539762260521377e-02,
	2.18694882948595424599e-02,
	8.86323982359930005737e-03,
	3.59207910759131235356e-03,
	1.45620945432529025516e-03,
	5.88041240820264096874e-04,
	2.46463134818469906812e-04,
	7.81794442939557092300e-05,
	7.14072491382608190305e-05,
	-1.85586374855275456654e-05,
	2.59073051863633712884e-05,
}

// kernelTan is __kernel_tan on [-pi/4, pi/4]: tan(x+y) if iy = 1, -1/tan(x+y) if iy = -1.
func kernelTan(x, y float64, iy int32) float64 {
	const (
		one    float64 = 1.00000000000000000000e+00
		pio4   float64 = 7.85398163397448278999e-01
		pio4lo float64 = 3.06161699786838301793e-17
	)
	T := &tanT
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	if ix < 0x3e300000 { // |x| < 2**-28
		if uint32(ix)|loWord(x)|uint32(iy+1) == 0 {
			return one / math.Abs(x)
		}
		if iy == 1 {
			return x
		}
		// compute -1 / (x+y) carefully
		w := x + y
		z := setLoWord(w, 0)
		v := y - (z - x)
		a := -one / w
		t := setLoWord(a, 0)
		s := one + float64(t*z)
		return t + float64(a*(s+float64(t*v)))
	}
	if ix >= 0x3fe59428 { // |x| >= 0.6744
		if hx < 0 {
			x = -x
			y = -y
		}
		z := pio4 - x
		w := pio4lo - y
		x = z + w
		y = 0.0
	}
	z := x * x
	w := z * z
	// x^5*(T[1]+x^4*T[3]+...+x^20*T[11]) + x^5*(x^2*(T[2]+x^4*T[4]+...+x^22*T[12]))
	r := T[1] + float64(w*(T[3]+float64(w*(T[5]+float64(w*(T[7]+float64(w*(T[9]+float64(w*T[11])))))))))
	v := float64(z * (T[2] + float64(w*(T[4]+float64(w*(T[6]+float64(w*(T[8]+float64(w*(T[10]+float64(w*T[12])))))))))))
	s := z * x
	r = y + float64(z*(float64(s*(r+v))+y))
	r += float64(T[0] * s)
	w = x + r
	if ix >= 0x3fe59428 {
		v = float64(iy)
		return float64(1-((hx>>30)&2)) * (v - float64(2.0*(x-((w*w)/(w+v)-r))))
	}
	if iy == 1 {
		return w
	}
	// compute -1.0 / (x+r) accurately
	z = setLoWord(w, 0)
	v = r - (z - x)
	a := -1.0 / w
	t := setLoWord(a, 0)
	s = 1.0 + float64(t*z)
	return t + float64(a*(s+float64(t*v)))
}

func Acos(x float64) float64 {
	const (
		one     float64 = 1.00000000000000000000e+00
		pi      float64 = 3.14159265358979311600e+00
		pio2_hi float64 = 1.57079632679489655800e+00
		pio2_lo float64 = 6.12323399573676603587e-17
		pS0     float64 = 1.66666666666666657415e-01
		pS1     float64 = -3.25565818622400915405e-01
		pS2     float64 = 2.01212532134862925881e-01
		pS3     float64 = -4.00555345006794114027e-02
		pS4     float64 = 7.91534994289814532176e-04
		pS5     float64 = 3.47933107596021167570e-05
		qS1     float64 = -2.40339491173441421878e+00
		qS2     float64 = 2.02094576023350569471e+00
		qS3     float64 = -6.88283971605453293030e-01
		qS4     float64 = 7.70381505559019352791e-02
	)
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	if ix >= 0x3ff00000 { // |x| >= 1
		if uint32(ix-0x3ff00000)|loWord(x) == 0 {
			if hx > 0 {
				return 0.0
			}
			return pi + 2.0*pio2_lo
		}
		return math.NaN()
	}
	if ix < 0x3fe00000 { // |x| < 0.5
		if ix <= 0x3c600000 { // |x| < 2**-57
			return pio2_hi + pio2_lo
		}
		z := x * x
		p := z * (pS0 + float64(z*(pS1+float64(z*(pS2+float64(z*(pS3+float64(z*(pS4+float64(z*pS5))))))))))
		q := one + float64(z*(qS1+float64(z*(qS2+float64(z*(qS3+float64(z*qS4)))))))
		r := p / q
		return pio2_hi - (x - (pio2_lo - float64(x*r)))
	}
	if hx < 0 { // x < -0.5
		z := (one + x) * 0.5
		p := z * (pS0 + float64(z*(pS1+float64(z*(pS2+float64(z*(pS3+float64(z*(pS4+float64(z*pS5))))))))))
		q := one + float64(z*(qS1+float64(z*(qS2+float64(z*(qS3+float64(z*qS4)))))))
		s := math.Sqrt(z)
		r := p / q
		w := float64(r*s) - pio2_lo
		return pi - float64(2.0*(s+w))
	}
	// x > 0.5
	z := float64((one - x) * 0.5)
	s := math.Sqrt(z)
	df := setLoWord(s, 0)
	c := (z - float64(df*df)) / (s + df)
	p := z * (pS0 + float64(z*(pS1+float64(z*(pS2+float64(z*(pS3+float64(z*(pS4+float64(z*pS5))))))))))
	q := one + float64(z*(qS1+float64(z*(qS2+float64(z*(qS3+float64(z*qS4)))))))
	r := p / q
	w := float64(r*s) + c
	return 2.0 * (df + w)
}

func Asin(x float64) float64 {
	const (
		one     float64 = 1.00000000000000000000e+00
		pio2_hi float64 = 1.57079632679489655800e+00
		pio2_lo float64 = 6.12323399573676603587e-17
		pio4_hi float64 = 7.85398163397448278999e-01
		pS0     float64 = 1.66666666666666657415e-01
		pS1     float64 = -3.25565818622400915405e-01
		pS2     float64 = 2.01212532134862925881e-01
		pS3     float64 = -4.00555345006794114027e-02
		pS4     float64 = 7.91534994289814532176e-04
		pS5     float64 = 3.47933107596021167570e-05
		qS1     float64 = -2.40339491173441421878e+00
		qS2     float64 = 2.02094576023350569471e+00
		qS3     float64 = -6.88283971605453293030e-01
		qS4     float64 = 7.70381505559019352791e-02
	)
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	if ix >= 0x3ff00000 { // |x| >= 1
		if uint32(ix-0x3ff00000)|loWord(x) == 0 {
			return float64(x*pio2_hi) + float64(x*pio2_lo)
		}
		return math.NaN()
	}
	if ix < 0x3fe00000 { // |x| < 0.5
		if ix < 0x3e400000 { // |x| < 2**-27
			return x
		}
		t := x * x
		p := t * (pS0 + float64(t*(pS1+float64(t*(pS2+float64(t*(pS3+float64(t*(pS4+float64(t*pS5))))))))))
		q := one + float64(t*(qS1+float64(t*(qS2+float64(t*(qS3+float64(t*qS4)))))))
		w := p / q
		return x + float64(x*w)
	}
	// 1 > |x| >= 0.5
	w := one - math.Abs(x)
	t := float64(w * 0.5)
	p := t * (pS0 + float64(t*(pS1+float64(t*(pS2+float64(t*(pS3+float64(t*(pS4+float64(t*pS5))))))))))
	q := one + float64(t*(qS1+float64(t*(qS2+float64(t*(qS3+float64(t*qS4)))))))
	s := math.Sqrt(t)
	if ix >= 0x3fef3333 { // |x| > 0.975
		w = p / q
		t = pio2_hi - (float64(2.0*(s+float64(s*w))) - pio2_lo)
	} else {
		w = setLoWord(s, 0)
		c := (t - float64(w*w)) / (s + w)
		r := p / q
		p = float64(2.0*s*r) - (pio2_lo - float64(2.0*c))
		q = pio4_hi - float64(2.0*w)
		t = pio4_hi - (p - q)
	}
	if hx > 0 {
		return t
	}
	return -t
}

var (
	atanhi = [...]float64{
		4.63647609000806093515e-01, // atan(0.5)hi
		7.85398163397448278999e-01, // atan(1.0)hi
		9.82793723247329054082e-01, // atan(1.5)hi
		1.57079632679489655800e+00, // atan(inf)hi
	}
	atanlo = [...]float64{
		2.26987774529616870924e-17, // atan(0.5)lo
		3.06161699786838301793e-17, // atan(1.0)lo
		1.39033110312309984516e-17, // atan(1.5)lo
		6.12323399573676603587e-17, // atan(inf)lo
	}
	aT = [...]float64{
		3.33333333333329318027e-01,
		-1.99999999998764832476e-01,
		1.42857142725034663711e-01,
		-1.11111104054623557880e-01,
		9.09088713343650656196e-02,
		-7.69187620504482999495e-02,
		6.66107313738753120669e-02,
		-5.83357013379057348645e-02,
		4.97687799461593236017e-02,
		-3.65315727442169155270e-02,
		1.62858201153657823623e-02,
	}
)

func Atan(x float64) float64 {
	const one float64 = 1.0
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	if ix >= 0x44100000 { // |x| >= 2^66
		if ix > 0x7ff00000 || (ix == 0x7ff00000 && loWord(x) != 0) {
			return x + x // NaN
		}
		if hx > 0 {
			return atanhi[3] + atanlo[3]
		}
		return -atanhi[3] - atanlo[3]
	}
	var id int
	if ix < 0x3fdc0000 { // |x| < 0.4375
		if ix < 0x3e400000 { // |x| < 2^-27
			return x
		}
		id = -1
	} else {
		x = math.Abs(x)
		if ix < 0x3ff30000 { // |x| < 1.1875
			if ix < 0x3fe60000 { // 7/16 <= |x| < 11/16
				id = 0
				x = (float64(2.0*x) - one) / (2.0 + x)
			} else { // 11/16 <= |x| < 19/16
				id = 1
				x = (x - one) / (x + one)
			}
		} else {
			if ix < 0x40038000 { // |x| < 2.4375
				id = 2
				x = (x - 1.5) / (one + float64(1.5*x))
			} else { // 2.4375 <= |x| < 2^66
				id = 3
				x = -1.0 / x
			}
		}
	}
	z := x * x
	w := z * z
	// break sum from i=0 to 10 aT[i]z**(i+1) into odd and even poly
	s1 := float64(z * (aT[0] + float64(w*(aT[2]+float64(w*(aT[4]+float64(w*(aT[6]+float64(w*(aT[8]+float64(w*aT[10])))))))))))
	s2 := float64(w * (aT[1] + float64(w*(aT[3]+float64(w*(aT[5]+float64(w*(aT[7]+float64(w*aT[9])))))))))
	if id < 0 {
		return x - float64(x*(s1+s2))
	}
	z = atanhi[id] - ((float64(x*(s1+s2)) - atanlo[id]) - x)
	if hx < 0 {
		return -z
	}
	return z
}

func Atan2(y, x float64) float64 {
	const (
		tiny   float64 = 1.0e-300
		pi_o_4 float64 = 7.8539816339744827900e-01
		pi_o_2 float64 = 1.5707963267948965580e+00
		pi     float64 = 3.1415926535897931160e+00
		pi_lo  float64 = 1.2246467991473531772e-16
	)
	hx, lx := words(x)
	ix := hx & 0x7fffffff
	hy, ly := words(y)
	iy := hy & 0x7fffffff
	if uint32(ix)|((lx|-lx)>>31) > 0x7ff00000 || uint32(iy)|((ly|-ly)>>31) > 0x7ff00000 {
		return x + y // x or y is NaN
	}
	if uint32(hx-0x3ff00000)|lx == 0 { // x = 1.0
		return Atan(y)
	}
	m := ((hy >> 31) & 1) | ((hx >> 30) & 2) // 2*sign(x)+sign(y)

	if uint32(iy)|ly == 0 { // y = 0
		switch m {
		case 0, 1:
			return y // atan(+-0,+anything)=+-0
		case 2:
			return pi + tiny // atan(+0,-anything) = pi
		default:
			return -pi - tiny // atan(-0,-anything) =-pi
		}
	}
	if uint32(ix)|lx == 0 { // x = 0
		if hy < 0 {
			return -pi_o_2 - tiny
		}
		return pi_o_2 + tiny
	}
	if ix == 0x7ff00000 { // x is INF
		if iy == 0x7ff00000 {
			switch m {
			case 0:
				return pi_o_4 + tiny
			case 1:
				return -pi_o_4 - tiny
			case 2:
				return 3.0*pi_o_4 + tiny
			default:
				return -3.0*pi_o_4 - tiny
			}
		}
		switch m {
		case 0:
			return 0
		case 1:
			return math.Copysign(0, -1)
		case 2:
			return pi + tiny
		default:
			return -pi - tiny
		}
	}
	if iy == 0x7ff00000 { // y is INF
		if hy < 0 {
			return -pi_o_2 - tiny
		}
		return pi_o_2 + tiny
	}

	k := (iy - ix) >> 20
	var z float64
	if k > 60 { // |y/x| > 2**60
		z = pi_o_2 + 0.5*pi_lo
		m &= 1
	} else if hx < 0 && k < -60 { // 0 > |y|/x > -2**-60
		z = 0.0
	} else {
		z = Atan(math.Abs(y / x))
	}
	switch m {
	case 0:
		return z // atan(+,+)
	case 1:
		return -z // atan(-,+)
	case 2:
		return pi - (z - pi_lo) // atan(+,-)
	default:
		return (z - pi_lo) - pi // atan(-,-)
	}
}
