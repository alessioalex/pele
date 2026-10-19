package js

import "math"

func Exp(x float64) float64 {
	const (
		one         float64 = 1.0
		o_threshold float64 = 7.09782712893383973096e+02
		u_threshold float64 = -7.45133219101941108420e+02
		invln2      float64 = 1.44269504088896338700e+00
		P1          float64 = 1.66666666666666019037e-01
		P2          float64 = -2.77777777770155933842e-03
		P3          float64 = 6.61375632143793436117e-05
		P4          float64 = -1.65339022054652515390e-06
		P5          float64 = 4.13813679705723846039e-08
		E           float64 = 2.718281828459045
		twom1000    float64 = 9.33263618503218878990e-302
		two1023     float64 = 8.988465674311579539e307
	)
	halF := [2]float64{0.5, -0.5}
	ln2HI := [2]float64{6.93147180369123816490e-01, -6.93147180369123816490e-01}
	ln2LO := [2]float64{1.90821492927058770002e-10, -1.90821492927058770002e-10}

	hx := uint32(hiWord(x))
	xsb := int32(hx >> 31) // sign bit of x
	hx &= 0x7fffffff       // high word of |x|

	// filter out non-finite argument
	if hx >= 0x40862e42 { // |x| >= 709.78...
		if hx >= 0x7ff00000 {
			if (hx&0xfffff)|loWord(x) != 0 {
				return x + x // NaN
			}
			if xsb == 0 {
				return x
			}
			return 0.0
		}
		if x > o_threshold {
			return math.Inf(1)
		}
		if x < u_threshold {
			return 0
		}
	}

	// argument reduction
	var hi, lo float64
	var k int32
	if hx > 0x3fd62e42 { // |x| > 0.5 ln2
		if hx < 0x3ff0a2b2 { // and |x| < 1.5 ln2
			// V8 special-cases exp(1) to return E.
			if x == 1.0 {
				return E
			}
			hi = x - ln2HI[xsb]
			lo = ln2LO[xsb]
			k = 1 - xsb - xsb
		} else {
			k = int32(float64(invln2*x) + halF[xsb])
			t := float64(k)
			hi = x - float64(t*ln2HI[0]) // t*ln2HI is exact here
			lo = float64(t * ln2LO[0])
		}
		x = hi - lo
	} else if hx < 0x3e300000 { // |x| < 2**-28
		return one + x
	}

	// x is now in primary range
	t := x * x
	var twopk float64
	if k >= -1021 {
		twopk = fromWords(0x3ff00000+(k<<20), 0)
	} else {
		twopk = fromWords(0x3ff00000+((k+1000)<<20), 0)
	}
	c := x - float64(t*(P1+float64(t*(P2+float64(t*(P3+float64(t*(P4+float64(t*P5)))))))))
	if k == 0 {
		return one - ((x*c)/(c-2.0) - x)
	}
	y := one - ((lo - (x*c)/(2.0-c)) - hi)
	if k >= -1021 {
		if k == 1024 {
			return float64(y * 2.0 * two1023)
		}
		return float64(y * twopk)
	}
	return float64(y * twopk * twom1000)
}

func Expm1(x float64) float64 {
	const (
		one         float64 = 1.0
		tiny        float64 = 1.0e-300
		o_threshold float64 = 7.09782712893383973096e+02
		ln2_hi      float64 = 6.93147180369123816490e-01
		ln2_lo      float64 = 1.90821492927058770002e-10
		invln2      float64 = 1.44269504088896338700e+00
		// Scaled Q's: Qn_here = 2**n * Qn_above, for R(2*z) where z = hxs = x*x/2
		Q1 float64 = -3.33333333333331316428e-02
		Q2 float64 = 1.58730158725481460165e-03
		Q3 float64 = -7.93650757867487942473e-05
		Q4 float64 = 4.00821782732936239552e-06
		Q5 float64 = -2.01099218183624371326e-07
	)
	hx := uint32(hiWord(x))
	xsb := hx & 0x80000000 // sign bit of x
	hx &= 0x7fffffff       // high word of |x|

	// filter out huge and non-finite argument
	if hx >= 0x4043687a { // |x| >= 56*ln2
		if hx >= 0x40862e42 { // |x| >= 709.78...
			if hx >= 0x7ff00000 {
				if (hx&0xfffff)|loWord(x) != 0 {
					return x + x // NaN
				}
				if xsb == 0 {
					return x
				}
				return -1.0
			}
			if x > o_threshold {
				return math.Inf(1)
			}
		}
		if xsb != 0 && x+tiny < 0.0 { // x < -56*ln2
			return tiny - one
		}
	}

	// argument reduction
	var hi, lo, c float64
	var k int32
	if hx > 0x3fd62e42 { // |x| > 0.5 ln2
		if hx < 0x3ff0a2b2 { // and |x| < 1.5 ln2
			if xsb == 0 {
				hi = x - ln2_hi
				lo = ln2_lo
				k = 1
			} else {
				hi = x + ln2_hi
				lo = -ln2_lo
				k = -1
			}
		} else {
			h := 0.5
			if xsb != 0 {
				h = -0.5
			}
			k = int32(float64(invln2*x) + h)
			t := float64(k)
			hi = x - float64(t*ln2_hi) // t*ln2_hi is exact here
			lo = float64(t * ln2_lo)
		}
		x = hi - lo
		c = (hi - x) - lo
	} else if hx < 0x3c900000 { // |x| < 2**-54
		return x
	}

	// x is now in primary range
	hfx := 0.5 * x
	hxs := float64(x * hfx)
	r1 := one + float64(hxs*(Q1+float64(hxs*(Q2+float64(hxs*(Q3+float64(hxs*(Q4+float64(hxs*Q5)))))))))
	t := 3.0 - float64(r1*hfx)
	e := float64(hxs * ((r1 - t) / (6.0 - float64(x*t))))
	if k == 0 {
		return x - (float64(x*e) - hxs) // c is 0
	}
	twopk := fromWords(0x3ff00000+(k<<20), 0) // 2^k
	e = float64(x*(e-c)) - c
	e -= hxs
	if k == -1 {
		return float64(0.5*(x-e)) - 0.5
	}
	if k == 1 {
		if x < -0.25 {
			return -2.0 * (e - (x + 0.5))
		}
		return one + float64(2.0*(x-e))
	}
	if k <= -2 || k > 56 { // suffice to return exp(x)-1
		y := one - (e - x)
		if k == 1024 {
			y = float64(y * 2.0 * 8.98846567431158e+307)
		} else {
			y = float64(y * twopk)
		}
		return y - one
	}
	var y float64
	if k < 20 {
		t = fromWords(0x3ff00000-(0x200000>>k), 0) // 1-2^-k
		y = t - (e - x)
		y = y * twopk
	} else {
		t = fromWords((0x3ff-k)<<20, 0) // 2^-k
		y = x - (e + t)
		y += one
		y = y * twopk
	}
	return y
}

func Log(x float64) float64 {
	const (
		ln2_hi float64 = 6.93147180369123816490e-01
		ln2_lo float64 = 1.90821492927058770002e-10
		two54  float64 = 1.80143985094819840000e+16
		Lg1    float64 = 6.666666666666735130e-01
		Lg2    float64 = 3.999999999940941908e-01
		Lg3    float64 = 2.857142874366239149e-01
		Lg4    float64 = 2.222219843214978396e-01
		Lg5    float64 = 1.818357216161805012e-01
		Lg6    float64 = 1.531383769920937332e-01
		Lg7    float64 = 1.479819860511658591e-01
		zero   float64 = 0.0
	)
	hx, lx := words(x)
	k := int32(0)
	if hx < 0x00100000 { // x < 2**-1022
		if uint32(hx&0x7fffffff)|lx == 0 {
			return math.Inf(-1) // log(+-0)=-inf
		}
		if hx < 0 {
			return math.NaN() // log(-#) = NaN
		}
		k -= 54
		x *= two54 // subnormal number, scale up x
		hx = hiWord(x)
	}
	if hx >= 0x7ff00000 {
		return x + x
	}
	k += (hx >> 20) - 1023
	hx &= 0x000fffff
	i := (hx + 0x95f64) & 0x100000
	x = setHiWord(x, hx|(i^0x3ff00000)) // normalize x or x/2
	k += i >> 20
	f := x - 1.0
	if (0x000fffff & (2 + hx)) < 3 { // -2**-20 <= f < 2**-20
		if f == zero {
			if k == 0 {
				return zero
			}
			dk := float64(k)
			return float64(dk*ln2_hi) + float64(dk*ln2_lo)
		}
		R := float64(f * f * (0.5 - float64(0.33333333333333333*f)))
		if k == 0 {
			return f - R
		}
		dk := float64(k)
		return float64(dk*ln2_hi) - ((R - float64(dk*ln2_lo)) - f)
	}
	s := f / (2.0 + f)
	dk := float64(k)
	z := s * s
	i = hx - 0x6147a
	w := z * z
	j := 0x6b851 - hx
	t1 := float64(w * (Lg2 + float64(w*(Lg4+float64(w*Lg6)))))
	t2 := float64(z * (Lg1 + float64(w*(Lg3+float64(w*(Lg5+float64(w*Lg7)))))))
	i |= j
	R := t2 + t1
	if i > 0 {
		hfsq := float64(0.5 * f * f)
		if k == 0 {
			return f - (hfsq - float64(s*(hfsq+R)))
		}
		return float64(dk*ln2_hi) - ((hfsq - (float64(s*(hfsq+R)) + float64(dk*ln2_lo))) - f)
	}
	if k == 0 {
		return f - float64(s*(f-R))
	}
	return float64(dk*ln2_hi) - ((float64(s*(f-R)) - float64(dk*ln2_lo)) - f)
}

func Log1p(x float64) float64 {
	const (
		ln2_hi float64 = 6.93147180369123816490e-01
		ln2_lo float64 = 1.90821492927058770002e-10
		Lp1    float64 = 6.666666666666735130e-01
		Lp2    float64 = 3.999999999940941908e-01
		Lp3    float64 = 2.857142874366239149e-01
		Lp4    float64 = 2.222219843214978396e-01
		Lp5    float64 = 1.818357216161805012e-01
		Lp6    float64 = 1.531383769920937332e-01
		Lp7    float64 = 1.479819860511658591e-01
		zero   float64 = 0.0
	)
	hx := hiWord(x)
	ax := hx & 0x7fffffff

	var f, c float64
	var hu int32
	k := int32(1)
	if hx < 0x3fda827a { // 1+x < sqrt(2)+
		if ax >= 0x3ff00000 { // x <= -1.0
			if x == -1.0 {
				return math.Inf(-1)
			}
			return math.NaN()
		}
		if ax < 0x3e200000 { // |x| < 2**-29
			if ax < 0x3c900000 { // |x| < 2**-54
				return x
			}
			return x - float64(x*x*0.5)
		}
		if hx > 0 || hx <= -0x402d413c { // sqrt(2)/2- <= 1+x < sqrt(2)+; int32(0xbfd2bec4)
			k = 0
			f = x
			hu = 1
		}
	}
	if hx >= 0x7ff00000 {
		return x + x
	}
	if k != 0 {
		var u float64
		if hx < 0x43400000 {
			u = 1.0 + x
			hu = hiWord(u)
			k = (hu >> 20) - 1023
			if k > 0 { // correction term
				c = 1.0 - (u - x)
			} else {
				c = x - (u - 1.0)
			}
			c /= u
		} else {
			u = x
			hu = hiWord(u)
			k = (hu >> 20) - 1023
			c = 0
		}
		hu &= 0x000fffff
		if hu < 0x6a09e { // u ~< sqrt(2)
			u = setHiWord(u, hu|0x3ff00000) // normalize u
		} else {
			k++
			u = setHiWord(u, hu|0x3fe00000) // normalize u/2
			hu = (0x00100000 - hu) >> 2
		}
		f = u - 1.0
	}
	dk := float64(k)
	hfsq := float64(0.5 * f * f)
	if hu == 0 { // |f| < 2**-20
		if f == zero {
			if k == 0 {
				return zero
			}
			c += float64(dk * ln2_lo)
			return float64(dk*ln2_hi) + c
		}
		R := float64(hfsq * (1.0 - float64(0.66666666666666666*f)))
		if k == 0 {
			return f - R
		}
		return float64(dk*ln2_hi) - ((R - (float64(dk*ln2_lo) + c)) - f)
	}
	s := f / (2.0 + f)
	z := s * s
	R := float64(z * (Lp1 + float64(z*(Lp2+float64(z*(Lp3+float64(z*(Lp4+float64(z*(Lp5+float64(z*(Lp6+float64(z*Lp7)))))))))))))
	if k == 0 {
		return f - (hfsq - float64(s*(hfsq+R)))
	}
	return float64(dk*ln2_hi) - ((hfsq - (float64(s*(hfsq+R)) + (float64(dk*ln2_lo) + c))) - f)
}

// kLog1p is k_log1p: log(1+f) - f for 1+f in ~[sqrt(2)/2, sqrt(2)].
func kLog1p(f float64) float64 {
	const (
		Lg1 float64 = 6.666666666666735130e-01
		Lg2 float64 = 3.999999999940941908e-01
		Lg3 float64 = 2.857142874366239149e-01
		Lg4 float64 = 2.222219843214978396e-01
		Lg5 float64 = 1.818357216161805012e-01
		Lg6 float64 = 1.531383769920937332e-01
		Lg7 float64 = 1.479819860511658591e-01
	)
	s := f / (2.0 + f)
	z := s * s
	w := z * z
	t1 := float64(w * (Lg2 + float64(w*(Lg4+float64(w*Lg6)))))
	t2 := float64(z * (Lg1 + float64(w*(Lg3+float64(w*(Lg5+float64(w*Lg7)))))))
	R := t2 + t1
	hfsq := float64(0.5 * f * f)
	return float64(s * (hfsq + R))
}

func Log2(x float64) float64 {
	const (
		two54   float64 = 1.80143985094819840000e+16
		ivln2hi float64 = 1.44269504072144627571e+00
		ivln2lo float64 = 1.67517131648865118353e-10
	)
	hx, lx := words(x)
	k := int32(0)
	if hx < 0x00100000 { // x < 2**-1022
		if uint32(hx&0x7fffffff)|lx == 0 {
			return math.Inf(-1)
		}
		if hx < 0 {
			return math.NaN()
		}
		k -= 54
		x *= two54 // subnormal number, scale up x
		hx = hiWord(x)
	}
	if hx >= 0x7ff00000 {
		return x + x
	}
	if hx == 0x3ff00000 && lx == 0 {
		return 0.0 // log(1) = +0
	}
	k += (hx >> 20) - 1023
	hx &= 0x000fffff
	i := (hx + 0x95f64) & 0x100000
	x = setHiWord(x, hx|(i^0x3ff00000)) // normalize x or x/2
	k += i >> 20
	y := float64(k)
	f := x - 1.0
	hfsq := float64(0.5 * f * f)
	r := kLog1p(f)

	// f-hfsq must (for args near 1) be evaluated in extra precision; see e_log2.c.
	hi := setLoWord(f-hfsq, 0)
	lo := (f - hi) - hfsq + r
	val_hi := float64(hi * ivln2hi)
	val_lo := float64((lo+hi)*ivln2lo) + float64(lo*ivln2hi)

	// spadd(val_hi, val_lo, y)
	w := y + val_hi
	val_lo += (y - w) + val_hi
	val_hi = w

	return val_lo + val_hi
}

func Log10(x float64) float64 {
	const (
		two54     float64 = 1.80143985094819840000e+16
		ivln10    float64 = 4.34294481903251816668e-01
		log10_2hi float64 = 3.01029995663611771306e-01
		log10_2lo float64 = 3.69423907715893078616e-13
	)
	hx, lx := words(x)
	k := int32(0)
	if hx < 0x00100000 { // x < 2**-1022
		if uint32(hx&0x7fffffff)|lx == 0 {
			return math.Inf(-1)
		}
		if hx < 0 {
			return math.NaN()
		}
		k -= 54
		x *= two54 // subnormal number, scale up x
		hx, lx = words(x)
	}
	if hx >= 0x7ff00000 {
		return x + x
	}
	if hx == 0x3ff00000 && lx == 0 {
		return 0.0 // log(1) = +0
	}
	k += (hx >> 20) - 1023

	i := int32(uint32(k) >> 31)
	hx = (hx & 0x000fffff) | ((0x3ff - i) << 20)
	y := float64(k + i)
	x = fromWords(hx, lx)

	z := float64(y*log10_2lo) + float64(ivln10*Log(x))
	return z + float64(y*log10_2hi)
}

func Cbrt(x float64) float64 {
	const (
		B1 uint32 = 715094163 // (1023-1023/3-0.03306235651)*2**20
		B2 uint32 = 696219795 // (1023-1023/3-54/3-0.03306235651)*2**20

		// |1/cbrt(x) - p(x)| < 2**-23.5 (~[-7.93e-8, 7.929e-8]).
		P0 float64 = 1.87595182427177009643
		P1 float64 = -1.88497979543377169875
		P2 float64 = 1.621429720105354466140
		P3 float64 = -0.758397934778766047437
		P4 float64 = 0.145996192886612446982
	)
	h, low := words(x)
	sign := uint32(h) & 0x80000000
	hx := uint32(h) ^ sign
	if hx >= 0x7ff00000 {
		return x + x // cbrt(NaN,INF) is itself
	}

	// rough cbrt to 5 bits
	var t float64
	if hx < 0x00100000 { // zero or subnormal
		if hx|low == 0 {
			return x // cbrt(0) is itself
		}
		t = fromWords(0x43500000, 0) // 2**54
		t *= x
		high := uint32(hiWord(t))
		t = fromWords(int32(sign|((high&0x7fffffff)/3+B2)), 0)
	} else {
		t = fromWords(int32(sign|(hx/3+B1)), 0)
	}

	// new cbrt to 23 bits
	r := (t * t) * (t / x)
	t = t * ((P0 + float64(r*(P1+float64(r*P2)))) + float64(((r*r)*r)*(P3+float64(r*P4))))

	// round t away from zero to 23 bits
	t = math.Float64frombits((math.Float64bits(t) + 0x80000000) & 0xffffffffc0000000)

	// one step Newton iteration to 53 bits with error < 0.667 ulps
	s := t * t              // t*t is exact
	r = x / s               // error <= 0.5 ulps; |r| < |t|
	w := t + t              // t+t is exact
	r = (r - t) / (w + r)   // r-t is exact; w+r ~= 3*t
	return t + float64(t*r) // error <= 0.5 + 0.5/3 + epsilon
}

func Sinh(x float64) float64 {
	const (
		KSINH_OVERFLOW float64 = 710.4758600739439
		TWO_M28        float64 = 3.725290298461914e-9 // 2^-28, empty lower half
		LOG_MAXD       float64 = 709.7822265625       // 0x40862E42 00000000, empty lower half
		shuge          float64 = 1.0e307
	)
	h := 0.5
	if x < 0 {
		h = -0.5
	}
	// |x| in [0, 22]. return sign(x)*0.5*(E+E/(E+1))
	ax := math.Abs(x)
	if ax < 22 {
		if ax < TWO_M28 { // sinh(x) = x
			return x
		}
		t := Expm1(ax)
		if ax < 1 {
			return h * (float64(2*t) - t*t/(t+1))
		}
		return h * (t + t/(t+1))
	}
	// |x| in [22, log(maxdouble)], return 0.5 * exp(|x|)
	if ax < LOG_MAXD {
		return h * Exp(ax)
	}
	// |x| in [log(maxdouble), overflowthreshold]
	if ax <= KSINH_OVERFLOW {
		w := Exp(0.5 * ax)
		t := h * w
		return float64(t * w)
	}
	// |x| > overflowthreshold or is NaN.
	return x * shuge
}

func Cosh(x float64) float64 {
	const (
		KCOSH_OVERFLOW float64 = 710.4758600739439
		one            float64 = 1.0
		half           float64 = 0.5
	)
	ix := hiWord(x) & 0x7fffffff

	// |x| in [0,0.5*log2], return 1+expm1(|x|)^2/(2*exp(|x|))
	if ix < 0x3fd62e43 {
		t := Expm1(math.Abs(x))
		w := one + t
		if ix < 0x3c800000 { // cosh(x) = 1 for |x| < 2^-55
			return w
		}
		return one + (t*t)/(w+w)
	}
	// |x| in [0.5*log2, 22], return (exp(|x|)+1/exp(|x|)/2
	if ix < 0x40360000 {
		t := Exp(math.Abs(x))
		return float64(half*t) + half/t
	}
	// |x| in [22, log(maxdouble)], return half*exp(|x|)
	if ix < 0x40862e42 {
		return half * Exp(math.Abs(x))
	}
	// |x| in [log(maxdouble), overflowthreshold]
	if math.Abs(x) <= KCOSH_OVERFLOW {
		w := Exp(half * math.Abs(x))
		t := half * w
		return float64(t * w)
	}
	if ix >= 0x7ff00000 { // x is INF or NaN
		return x * x
	}
	return math.Inf(1)
}

func Tanh(x float64) float64 {
	const (
		tiny float64 = 1.0e-300
		one  float64 = 1.0
		two  float64 = 2.0
	)
	jx := hiWord(x)
	ix := jx & 0x7fffffff
	if ix >= 0x7ff00000 { // x is INF or NaN
		if jx >= 0 {
			return one/x + one // tanh(+-inf)=+-1
		}
		return one/x - one // tanh(NaN) = NaN
	}
	var z float64
	if ix < 0x40360000 { // |x| < 22
		if ix < 0x3e300000 { // |x| < 2**-28
			return x
		}
		if ix >= 0x3ff00000 { // |x| >= 1
			t := Expm1(two * math.Abs(x))
			z = one - two/(t+two)
		} else {
			t := Expm1(-two * math.Abs(x))
			z = -t / (t + two)
		}
	} else { // |x| >= 22, return +-1
		z = one - tiny
	}
	if jx >= 0 {
		return z
	}
	return -z
}

// Pow is Math.pow and the ** operator: fdlibm's e_pow.c as V8 has it, where
// (+-1)**(+-Infinity) and 1**NaN are NaN, unlike C.
func Pow(x, y float64) float64 {
	const (
		zero  float64 = 0.0
		one   float64 = 1.0
		two   float64 = 2.0
		two53 float64 = 9007199254740992.0
		huge  float64 = 1.0e300
		tiny  float64 = 1.0e-300
		// poly coefs for (3/2)*(log(x)-2s-2/3*s**3
		L1      float64 = 5.99999999999994648725e-01
		L2      float64 = 4.28571428578550184252e-01
		L3      float64 = 3.33333329818377432918e-01
		L4      float64 = 2.72728123808534006489e-01
		L5      float64 = 2.30660745775561754067e-01
		L6      float64 = 2.06975017800338417784e-01
		P1      float64 = 1.66666666666666019037e-01
		P2      float64 = -2.77777777770155933842e-03
		P3      float64 = 6.61375632143793436117e-05
		P4      float64 = -1.65339022054652515390e-06
		P5      float64 = 4.13813679705723846039e-08
		lg2     float64 = 6.93147180559945286227e-01
		lg2_h   float64 = 6.93147182464599609375e-01
		lg2_l   float64 = -1.90465429995776804525e-09
		ovt     float64 = 8.0085662595372944372e-0017 // -(1024-log2(ovfl+.5ulp))
		cp      float64 = 9.61796693925975554329e-01  // 2/(3ln2)
		cp_h    float64 = 9.61796700954437255859e-01  // (float)cp
		cp_l    float64 = -7.02846165095275826516e-09 // tail of cp_h
		ivln2   float64 = 1.44269504088896338700e+00  // 1/ln2
		ivln2_h float64 = 1.44269502162933349609e+00  // 24b 1/ln2
		ivln2_l float64 = 1.92596299112661746887e-08  // 1/ln2 tail
	)
	bp := [2]float64{1.0, 1.5}
	dp_h := [2]float64{0.0, 5.84962487220764160156e-01}
	dp_l := [2]float64{0.0, 1.35003920212974897128e-08}

	hx, lx := words(x)
	hy, ly := words(y)
	ix := hx & 0x7fffffff
	iy := hy & 0x7fffffff

	// y==zero: x**0 = 1
	if uint32(iy)|ly == 0 {
		return one
	}

	// +-NaN return x+y
	if ix > 0x7ff00000 || (ix == 0x7ff00000 && lx != 0) || iy > 0x7ff00000 || (iy == 0x7ff00000 && ly != 0) {
		return x + y
	}

	// determine if y is an odd int when x < 0
	// yisint = 0 ... y is not an integer
	// yisint = 1 ... y is an odd int
	// yisint = 2 ... y is an even int
	yisint := int32(0)
	if hx < 0 {
		if iy >= 0x43400000 {
			yisint = 2 // even integer y
		} else if iy >= 0x3ff00000 {
			k := (iy >> 20) - 0x3ff // exponent
			if k > 20 {
				j := ly >> (52 - k)
				if j<<(52-k) == ly {
					yisint = 2 - int32(j&1)
				}
			} else if ly == 0 {
				j := iy >> (20 - k)
				if j<<(20-k) == iy {
					yisint = 2 - (j & 1)
				}
			}
		}
	}

	// special value of y
	if ly == 0 {
		if iy == 0x7ff00000 { // y is +-inf
			if uint32(ix-0x3ff00000)|lx == 0 {
				return y - y // (+-1)**+-inf is NaN
			} else if ix >= 0x3ff00000 { // (|x|>1)**+-inf = inf,0
				if hy >= 0 {
					return y
				}
				return zero
			}
			// (|x|<1)**-,+inf = inf,0
			if hy < 0 {
				return -y
			}
			return zero
		}
		if iy == 0x3ff00000 { // y is +-1
			if hy < 0 {
				return one / x
			}
			return x
		}
		if hy == 0x40000000 { // y is 2
			return float64(x * x)
		}
		if hy == 0x3fe00000 && hx >= 0 { // y is 0.5, x >= +0
			return math.Sqrt(x)
		}
	}

	ax := math.Abs(x)
	// special value of x
	if lx == 0 && (ix == 0x7ff00000 || ix == 0 || ix == 0x3ff00000) { // x is +-0,+-inf,+-1
		z := ax
		if hy < 0 {
			z = one / z // z = (1/|x|)
		}
		if hx < 0 {
			if (ix-0x3ff00000)|yisint == 0 {
				z = math.NaN() // (-1)**non-int is NaN
			} else if yisint == 1 {
				z = -z // (x<0)**odd = -(|x|**odd)
			}
		}
		return z
	}

	n := (hx >> 31) + 1

	// (x<0)**(non-int) is NaN
	if n|yisint == 0 {
		return math.NaN()
	}

	s := one // s (sign of result -ve**odd) = -1 else = 1
	if n|(yisint-1) == 0 {
		s = -one // (-ve)**(odd int)
	}

	var t1, t2 float64
	if iy > 0x41e00000 { // |y| > 2**31
		if iy > 0x43f00000 { // |y| > 2**64, must o/uflow
			if ix <= 0x3fefffff {
				if hy < 0 {
					return math.Inf(1)
				}
				return 0
			}
			if ix >= 0x3ff00000 {
				if hy > 0 {
					return math.Inf(1)
				}
				return 0
			}
		}
		// over/underflow if x is not close to one
		if ix < 0x3fefffff {
			if hy < 0 {
				return s * huge * huge
			}
			return s * tiny * tiny
		}
		if ix > 0x3ff00000 {
			if hy > 0 {
				return s * huge * huge
			}
			return s * tiny * tiny
		}
		// now |1-x| is tiny <= 2**-20, suffice to compute
		// log(x) by x-x^2/2+x^3/3-x^4/4
		t := ax - one // t has 20 trailing zeros
		w := (t * t) * (0.5 - float64(t*(0.3333333333333333333333-float64(t*0.25))))
		u := float64(ivln2_h * t) // ivln2_h has 21 sig. bits
		v := float64(t*ivln2_l) - float64(w*ivln2)
		t1 = setLoWord(u+v, 0)
		t2 = v - (t1 - u)
	} else {
		n = 0
		// take care subnormal number
		if ix < 0x00100000 {
			ax *= two53
			n -= 53
			ix = hiWord(ax)
		}
		n += (ix >> 20) - 0x3ff
		j := ix & 0x000fffff
		// determine interval
		ix = j | 0x3ff00000 // normalize ix
		var k int32
		if j <= 0x3988e {
			k = 0 // |x|<sqrt(3/2)
		} else if j < 0xbb67a {
			k = 1 // |x|<sqrt(3)
		} else {
			k = 0
			n++
			ix -= 0x00100000
		}
		ax = setHiWord(ax, ix)

		// compute ss = s_h+s_l = (x-1)/(x+1) or (x-1.5)/(x+1.5)
		u := ax - bp[k]
		v := one / (ax + bp[k])
		ss := float64(u * v)
		s_h := setLoWord(ss, 0)
		// t_h=ax+bp[k] High
		t_h := fromWords(((ix>>1)|0x20000000)+0x00080000+(k<<18), 0)
		t_l := ax - (t_h - bp[k])
		s_l := v * ((u - float64(s_h*t_h)) - float64(s_h*t_l))
		// compute log(ax)
		s2 := ss * ss
		r := float64(s2 * s2 * (L1 + float64(s2*(L2+float64(s2*(L3+float64(s2*(L4+float64(s2*(L5+float64(s2*L6)))))))))))
		r += float64(s_l * (s_h + ss))
		s2 = float64(s_h * s_h)
		t_h = setLoWord(3.0+s2+r, 0)
		t_l = r - ((t_h - 3.0) - s2)
		// u+v = ss*(1+...)
		u = float64(s_h * t_h)
		v = float64(s_l*t_h) + float64(t_l*ss)
		// 2/(3log2)*(ss+...)
		p_h := setLoWord(u+v, 0)
		p_l := v - (p_h - u)
		z_h := float64(cp_h * p_h) // cp_h+cp_l = 2/(3*log2)
		z_l := float64(cp_l*p_h) + float64(p_l*cp) + dp_l[k]
		// log2(ax) = (ss+..)*2/(3*log2) = n + dp_h + z_h + z_l
		t := float64(n)
		t1 = setLoWord(((z_h+z_l)+dp_h[k])+t, 0)
		t2 = z_l - (((t1 - t) - dp_h[k]) - z_h)
	}

	// split up y into y1+y2 and compute (y1+y2)*(t1+t2)
	y1 := setLoWord(y, 0)
	p_l := float64((y-y1)*t1) + float64(y*t2)
	p_h := float64(y1 * t1)
	z := p_l + p_h
	j, i := words(z)
	if j >= 0x40900000 { // z >= 1024
		if uint32(j-0x40900000)|i != 0 { // if z > 1024
			return s * huge * huge // overflow
		}
		if p_l+ovt > z-p_h {
			return s * huge * huge // overflow
		}
	} else if j&0x7fffffff >= 0x4090cc00 { // z <= -1075
		if (uint32(j)-0xc090cc00)|i != 0 { // z < -1075
			return s * tiny * tiny // underflow
		}
		if p_l <= z-p_h {
			return s * tiny * tiny // underflow
		}
	}

	// compute 2**(p_h+p_l)
	ia := j & 0x7fffffff
	k := (ia >> 20) - 0x3ff
	n = 0
	if ia > 0x3fe00000 { // if |z| > 0.5, set n = [z+0.5]
		n = j + (0x00100000 >> (k + 1))
		k = ((n & 0x7fffffff) >> 20) - 0x3ff // new k for n
		t := fromWords(n&^(0x000fffff>>k), 0)
		n = ((n & 0x000fffff) | 0x00100000) >> (20 - k)
		if j < 0 {
			n = -n
		}
		p_h -= t
	}
	t := setLoWord(p_l+p_h, 0)
	u := float64(t * lg2_h)
	v := float64((p_l-(t-p_h))*lg2) + float64(t*lg2_l)
	z = u + v
	w := v - (z - u)
	t = z * z
	t1 = z - float64(t*(P1+float64(t*(P2+float64(t*(P3+float64(t*(P4+float64(t*P5)))))))))
	r := (z * t1) / ((t1 - two) - (w + float64(z*w)))
	z = one - (r - z)
	j = hiWord(z)
	j += n << 20
	if (j >> 20) <= 0 {
		z = scalbn(z, n) // subnormal output
	} else {
		z = setHiWord(z, hiWord(z)+(n<<20))
	}
	return s * z
}
