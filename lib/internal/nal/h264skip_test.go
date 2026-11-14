package nal

import "testing"

// A skip slice parses as a P slice of the frame_num and picture order
// count it was made with, under its PPS, a frame picture in a sequence
// of fields or of macroblock-adaptive frames, and its slice data skips
// every macroblock of the frame and ends there.
func TestH264SkipSliceParses(t *testing.T) {
	h := H264{ProfileIdc: 100, LevelIdc: 41, WidthMbs: 21, HeightMbs: 16, Log2MaxFrameNum: 6, MaxNumRefFrames: 4, CABAC: true, InitQP: 20}
	for _, c := range []struct {
		pocType      uint8
		frameMbsOnly bool
		mbaff        bool
	}{{0, true, false}, {1, true, false}, {2, true, false}, {0, false, false}, {0, false, true}, {2, false, true}} {
		pocType := c.pocType
		s := parseSPS(t, h.SPS())
		s.PicOrderCntType, s.Log2MaxPicOrderCntLsb = pocType, 8
		if !c.frameMbsOnly {
			s.FrameMbsOnly, s.MbAdaptiveFrameField, s.HeightMapUnits = false, c.mbaff, 8
		}
		var sps [32]*H264SPS
		sps[s.ID] = s
		var ppss [256]*H264PPS
		pps := H264SkipPPS(200, s.ID)
		ppss[pps.ID] = &pps
		unit := H264SkipSlice(s, &pps, 37, 74)
		rbsp := Unescape(nil, unit[1:])
		var sh H264SliceHeader
		if err := sh.Parse(rbsp, unit[0], &ppss, &sps); err != nil {
			t.Fatal(err)
		}
		wantLsb := uint32(0)
		if pocType == 0 {
			wantLsb = 74
		}
		if unit[0] != 0x21 || sh.SliceType != H264SliceP || sh.PPSID != 200 || sh.FrameNum != 37 || sh.FieldPic || sh.PicOrderCntLsb != wantLsb || sh.AdaptiveRefPicMarking || sh.DeltaPicOrderCnt != [2]int32{} {
			t.Errorf("%+v: NAL header %#x, slice header %+v", c, unit[0], sh)
		}
		r := NewReader(rbsp)
		r.UE()
		r.UE()
		r.UE()
		r.Skip(int(s.Log2MaxFrameNum))
		if !c.frameMbsOnly && r.Flag() {
			t.Errorf("%+v: field_pic_flag set", c)
		}
		switch pocType {
		case 0:
			r.Skip(int(s.Log2MaxPicOrderCntLsb))
		case 1:
			r.SE()
		}
		if override, modification, adaptive := r.Flag(), r.Flag(), r.Flag(); override || modification || adaptive {
			t.Errorf("picture order count type %d: reference count override %v, list modification %v, adaptive marking %v", pocType, override, modification, adaptive)
		}
		if qp, deblock := r.SE(), r.UE(); qp != 0 || deblock != 1 {
			t.Errorf("picture order count type %d: slice_qp_delta %d, disable_deblocking_filter_idc %d", pocType, qp, deblock)
		}
		if skip := r.UE(); skip != 21*16 || r.MoreRBSPData() || r.Short() {
			t.Errorf("%+v: mb_skip_run %d of %d macroblocks, more data %v", c, skip, 21*16, r.MoreRBSPData())
		}
	}
}
