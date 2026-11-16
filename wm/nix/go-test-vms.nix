{
  pkgs,
  lys-wm,
}: let
  gpuPackages = ["render/vulkan" "render/gles" "harness"];
  tests = lys-wm.overrideAttrs {
    pname = "lys-wm-vm-tests";
    goTests = map (p: "wm/internal/${p}") (gpuPackages ++ ["backend/drm"]);
    resources = ["wm/internal" "docs/wm/shaders"];
    doCheck = false;
    buildPhase = ''
      runHook preBuild
      mkdir -p "$out/bin"
      ${pkgs.lib.concatMapStringsSep "\n" (p: ''go test -c -o "$out/bin/${baseNameOf p}.test" ./wm/internal/${p}'') (gpuPackages ++ ["backend/drm"])}
      runHook postBuild
    '';
    installPhase = ''
      runHook preInstall
      mkdir -p "$out/wm" "$out/docs/wm"
      cp -R wm/internal "$out/wm/internal"
      cp -R docs/wm/shaders "$out/docs/wm/shaders"
      runHook postInstall
    '';
    postInstall = "";
    postFixup = "";
  };
  env = import ./test-env.nix pkgs;
in
  import ./vm-test.nix pkgs {
    name = "lys-wm-vms-go-tests";
    timeout = 900;
    nodes = {
      seat = {
        boot = {
          kernelModules = ["vkms" "udmabuf"];
          extraModprobeConfig = "options vkms enable_cursor=1 enable_overlay=1";
        };
        virtualisation = {
          graphics = true;
          cores = 2;
          memorySize = 2048;
          qemu.options = ["-vga none" "-device virtio-gpu-pci"];
        };
        environment = {
          variables = env;
          systemPackages = [pkgs.bpftrace];
        };
      };
      gpu = {
        boot.kernelModules = ["udmabuf"];
        virtualisation = {
          cores = 4;
          memorySize = 4096;
          diskSize = 8192;
        };
        systemd.coredump.settings.Coredump = {
          ProcessSizeMax = "6G";
          ExternalSizeMax = "6G";
        };
        environment = {
          variables = env;
          systemPackages = import ./test-programs.nix pkgs;
        };
      };
    };
    testScript = ''
      import re

      start_all()

      with subtest("seat: start the DRM seat tests"):
          seat.wait_for_unit("multi-user.target")
          seat.succeed("test -c /dev/udmabuf")
          seat.wait_until_succeeds("readlink /sys/class/drm/card*/device | grep -q /vkms$")
          seat.succeed(
              "bpftrace -q -e 'fexit:drm_plane_get_damage_clips /retval != 0/ { printf(\"clip %d %d %d %d\\n\", retval->x1, retval->y1, retval->x2, retval->y2); }' > /tmp/damage.log 2>&1 &"
          )
          seat.wait_until_succeeds("pgrep -x bpftrace && sleep 2")
          seat.succeed("echo 0x10 > /sys/module/drm/parameters/debug", "dmesg -C")
          seat.succeed(
              "(cd ${tests}/bin && LYS_WM_SEAT=1 HOME=/tmp TMPDIR=/tmp timeout 240 ./drm.test -test.count=1 -test.v -test.run 'OnASeat' > /tmp/seat.log 2>&1; "
              "echo $? > /tmp/seat.status) > /dev/null 2>&1 &"
          )

      def ran(name, command):
          status, _ = gpu.execute(command, timeout=300)
          if status != 0:
              print(gpu.execute(f"cat /tmp/{name}.log")[1])
              print(gpu.execute("coredumpctl info --no-pager")[1])
              raise Exception(f"the {name} tests failed with status {status}")

      def assert_ran(log, allowed):
          skipped = re.findall(r"--- SKIP: (\S+)", gpu.succeed(f"cat {log}"))
          unexpected = [s for s in skipped if not re.search(allowed, s)]
          assert not unexpected, f"these tests did not run: {unexpected}"

      def gpu_tests(name, dir, tests, allowed):
          ran(name, f"cd ${tests}/wm/internal/{dir} && LYS_WM_HEADLESS=1 GOTRACEBACK=crash HOME=/tmp TMPDIR=/tmp timeout 280 ${tests}/bin/{name}.test "
              f"-test.count=1 -test.v -test.run '{tests}' > /tmp/{name}.log 2>&1")
          print(gpu.succeed(f"grep -E '^(--- |    --- |ok|FAIL|PASS|.*_test.go:.*(frame|nested|async))' /tmp/{name}.log"))
          assert_ran(f"/tmp/{name}.log", allowed)

      with subtest("gpu-tests"):
          gpu.wait_for_unit("multi-user.target")
          gpu.succeed("test -c /dev/udmabuf")
          gpu_tests("vulkan", "render/vulkan", ".", "OnHardware")
          gpu_tests("gles", "render/gles", "AsyncRead", "^$")
          gpu_tests(
              "harness",
              "harness",
              "TestGlass|TestTheRim|TestMica|TestAHiddenBlur|TestABlurBehindAnOpaque|TestAClosingWindowKeeps|TestABlurRegion|TestPopupsBlurByTheirRegion|TestAPopupsFirstFrame|TestDecorationBlurReachesPopups|Nested(LysWMRendersWithVulkan|GLESNames|GPURenderers)|FrameCost|OnTheGPURenderers|TestADMABUFDestroyed|TestFullscreenDMABUFs|TestAnHDROutputScansOutOnlyWhatMatchesItsSignal|TestAFullscreenDMABUF|TestAnARGBDMABUF|TestATenBit|TestDestroyingAReplaced|TestAFullscreenSurface|TestATranslucentFullscreen|TestARoundedWindow|TestImageCopyCaptureIntoDMABUFs|TestACaptureIntoADMABUF|TestAFrameDestroyedWhile|TestScaledClientCursors|TestTenBitShm|TheVulkanRenderer|TestIntegerScale|TestAScreenShader|TestTheExampleShaders|TestTheLockScreenAndScreencopy|TestAGenie|TestScanoutStopsOnly|TestAWindowWhoseBufferIsGone",
              "OnHardware|TestScreencopyIntoDMABUFs|TestACaptureIntoADMABUF|TestNestedFrameCost/gles|TestAnARGBDMABUF|TestATenBitDMABUF",
          )

      with subtest("seat"):
          seat.wait_until_succeeds("test -s /tmp/seat.status", timeout=300)
          status = int(seat.succeed("cat /tmp/seat.status"))
          seat.succeed("echo 0 > /sys/module/drm/parameters/debug", "pkill -INT bpftrace || true")
          results = seat.succeed("cat /tmp/seat.log")
          print(results)
          assert status == 0, f"the seat tests failed with status {status}"
          skipped = re.findall(r"--- SKIP: (\S+)", results)
          assert not skipped, f"these seat tests did not run: {skipped}"
          kernel = seat.succeed("dmesg")
          requests = len(re.findall(r"Allocated atomic state", kernel))
          commits = len(re.findall(r"\] committing ", kernel))
          nonblocking = len(re.findall(r"\] committing \S+ nonblocking", kernel))
          print(f"drm.debug: {requests} atomic requests, {requests - commits} of them TEST_ONLY, {commits} commits of which {nonblocking} nonblocking")
          clips = [tuple(map(int, m)) for m in re.findall(r"clip (-?\d+) (-?\d+) (-?\d+) (-?\d+)", seat.succeed("cat /tmp/damage.log"))]
          print(f"damage clips the kernel holds for the planes: {clips}")
          assert clips.count((32, 32, 96, 96)) >= 6, f"the six small frames' clips did not reach the kernel: {clips}"
    '';
  }
