package compiled

import "testing"

func TestUnionDiscreteBounds(t *testing.T) {
	t.Parallel()

	type want struct {
		z intBounds
		r bool // representable
	}

	type testcase struct {
		name string
		x    intBounds
		y    intBounds
		want want
	}

	for _, tc := range []testcase{
		{
			name: "[0, ...] U [10, ...]",
			x:    intBounds{flags: hasMin, min: 0},
			y:    intBounds{flags: hasMin, min: 10},
			want: want{
				z: intBounds{flags: hasMin, min: 0},
				r: true,
			},
		},
		{
			name: "[10, ...] U [0, ...]",
			x:    intBounds{flags: hasMin, min: 10},
			y:    intBounds{flags: hasMin, min: 0},
			want: want{
				z: intBounds{flags: hasMin, min: 0},
				r: true,
			},
		},
		{
			name: "[..., 10] U [..., 100]",
			x:    intBounds{flags: hasMax, max: 10},
			y:    intBounds{flags: hasMax, max: 100},
			want: want{
				z: intBounds{flags: hasMax, max: 100},
				r: true,
			},
		},
		{
			name: "[..., 100] U [..., 10]",
			x:    intBounds{flags: hasMax, max: 100},
			y:    intBounds{flags: hasMax, max: 10},
			want: want{
				z: intBounds{flags: hasMax, max: 100},
				r: true,
			},
		},
		{
			name: "[0, 100] U [0, 10]",
			x:    intBounds{flags: hasMin | hasMax, min: 0, max: 100},
			y:    intBounds{flags: hasMin | hasMax, min: 0, max: 10},
			want: want{
				z: intBounds{flags: hasMin | hasMax, min: 0, max: 100},
				r: true,
			},
		},
		{
			name: "[0, 10] U [0, 100]",
			x:    intBounds{flags: hasMin | hasMax, min: 0, max: 10},
			y:    intBounds{flags: hasMin | hasMax, min: 0, max: 100},
			want: want{
				z: intBounds{flags: hasMin | hasMax, min: 0, max: 100},
				r: true,
			},
		},
		{
			name: "[..., 10] U [0, 100]",
			x:    intBounds{flags: hasMax, min: 0, max: 10},
			y:    intBounds{flags: hasMin | hasMax, min: 0, max: 100},
			want: want{
				z: intBounds{flags: hasMax, min: 0, max: 100},
				r: true,
			},
		},
		{
			name: "[0, 100] U [..., 10]",
			x:    intBounds{flags: hasMax, min: 0, max: 10},
			y:    intBounds{flags: hasMin | hasMax, min: 0, max: 100},
			want: want{
				z: intBounds{flags: hasMax, min: 0, max: 100},
				r: true,
			},
		},
		{
			name: "[-10, 10] U [10, 20]",
			x:    intBounds{flags: hasMin | hasMax, min: -10, max: 10},
			y:    intBounds{flags: hasMin | hasMax, min: 10, max: 20},
			want: want{
				z: intBounds{flags: hasMin | hasMax, min: -10, max: 20},
				r: true,
			},
		},
		{
			name: "[-10, -1] U [0, 20]",
			x:    intBounds{flags: hasMin | hasMax, min: -10, max: -1},
			y:    intBounds{flags: hasMin | hasMax, min: 0, max: 20},
			want: want{
				z: intBounds{flags: hasMin | hasMax, min: -10, max: 20},
				r: true,
			},
		},
		{
			name: "[..., N] U [N, ...]",
			x:    intBounds{flags: hasMax, max: 10},
			y:    intBounds{flags: hasMin, min: 10},
			want: want{
				z: intBounds{},
				r: true,
			},
		},
		{
			// NOTE(i4k): there's a hole
			name: "[-10, -1] U [1, 20]",
			x:    intBounds{flags: hasMin | hasMax, min: -10, max: -1},
			y:    intBounds{flags: hasMin | hasMax, min: 1, max: 20},
			want: want{
				r: false,
			},
		},
		{
			// NOTE(i4k): there's bigger hole
			name: "[-10, -1] U [10, 20]",
			x:    intBounds{flags: hasMin | hasMax, min: -10, max: -1},
			y:    intBounds{flags: hasMin | hasMax, min: 10, max: 20},
			want: want{
				r: false,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, representable := unionDiscreteBounds(tc.x, tc.y)
			if got != tc.want.z {
				t.Fatalf("got unexpected union: %#v, expected: %#v", got, tc.want.z)
			}
			if representable != tc.want.r {
				t.Fatalf("got unexpected representable flag: %t, expected: %t", representable, tc.want.r)
			}
		})
	}
}
