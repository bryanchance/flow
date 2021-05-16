#!/usr/bin/env sh
WORK_DIR=${WORK_DIR:-$NOMAD_TASK_DIR}
OUTPUT_DIR=${OUTPUT_DIR:-"${WORK_DIR}/render"}

RESOLUTION_X=${RESOLUTION_X:-1920}
RESOLUTION_Y=${RESOLUTION_Y:-1080}
RESOLUTION_SCALE=${RESOLUTION_SCALE:-100}
RENDER_SAMPLES=${RENDER_SAMPLES:-100}
RENDER_TILE_X=16
RENDER_TILE_Y=16
RENDER_SLICE_MIN_X=${RENDER_SLICE_MIN_X:-}
RENDER_SLICE_MAX_X=${RENDER_SLICE_MAX_X:-}
RENDER_SLICE_MIN_Y=${RENDER_SLICE_MIN_Y:-}
RENDER_SLICE_MAX_Y=${RENDER_SLICE_MAX_Y:-}
RENDER_DEVICE=${RENDER_DEVICE:-"CPU"}
RENDER_START_FRAME=${RENDER_START_FRAME:-"1"}
PROJECT_NAME=${PROJECT_NAME:-}
PROJECT_ID=${PROJECT_ID:-"$(date +%Y%m%d%H%M)"}
PROJECT_DIR=${WORK_DIR}/project
SCENE_EXTRA=""
OUTPUT_FILENAME_SUFFIX=""

GPU_ENABLED=0
EXTRA_CONFIG=""

RENDER_FRAME=$((RENDER_START_FRAME + NOMAD_ALLOC_INDEX))

echo "work dir: ${WORK_DIR}"
mkdir -p ${WORK_DIR}

if [ ! -z "$(which nvidia-smi)" ] && [ "${RENDER_DEVICE}" = "GPU" ]; then
    echo "enabling GPU rendering"
    GPU_ENABLED=1
else
    echo "using CPU rendering"
fi

cat <<EOF>${WORK_DIR}/render.py
import bpy
import os

prop = bpy.context.preferences.addons['cycles'].preferences
prop.get_devices()

# set output path
filename = bpy.path.basename(bpy.data.filepath)
filename = os.path.splitext(filename)[0]

if filename:
    current_dir = os.getcwd()

EOF

if [ "${GPU_ENABLED}" = 1 ]; then
    RENDER_TILE_X=256
    RENDER_TILE_Y=256
    RENDER_DEVICE="GPU"
    EXTRA="
prop.compute_device_type = 'OPTIX'
# configure gpu
for device in prop.devices:
    if device.type == 'OPTIX':
        device.use = True
"
fi

# calculate region
if [ ! -z "${RENDER_SLICE_MIN_X}" ] && [ ! -z "${RENDER_SLICE_MAX_X}" ] && [ ! -z "${RENDER_SLICE_MIN_Y}" ] && [ ! -z "${RENDER_SLICE_MAX_Y}" ]; then
    echo "rendering region: ${RENDER_SLICE_MIN_X}, ${RENDER_SLICE_MAX_X}, ${RENDER_SLICE_MIN_Y}, ${RENDER_SLICE_MAX_Y}"
    OUTPUT_FILENAME_SUFFIX="${RENDER_FRAME}_${NOMAD_ALLOC_ID:-$(uuidgen)}"
    SCENE_EXTRA="
    scene.render.border_min_x = ${RENDER_SLICE_MIN_X}
    scene.render.border_max_x = ${RENDER_SLICE_MAX_X}
    scene.render.border_min_y = ${RENDER_SLICE_MIN_Y}
    scene.render.border_max_y = ${RENDER_SLICE_MAX_Y}
    scene.render.use_border = True
"
fi

cat <<EOF>>${WORK_DIR}/render.py
${EXTRA}
for scene in bpy.data.scenes:
    scene.cycles.device = '${RENDER_DEVICE}'
    scene.cycles.samples = ${RENDER_SAMPLES}
    # configure resolution
    scene.render.resolution_x = ${RESOLUTION_X}
    scene.render.resolution_y = ${RESOLUTION_Y}
    scene.render.resolution_percentage = ${RESOLUTION_SCALE}
    scene.render.tile_x = ${RENDER_TILE_X}
    scene.render.tile_y = ${RENDER_TILE_Y}
    scene.render.filepath = os.path.join("${OUTPUT_DIR}", filename + "_" + "${OUTPUT_FILENAME_SUFFIX}")
    ${SCENE_EXTRA}
EOF

echo "render script:"
echo "----------"
cat ${WORK_DIR}/render.py
echo "----------"

project_dir=$(dirname $(find ${PROJECT_DIR} -name "*.blend" | head -1))
project_file=$(basename $(find ${PROJECT_DIR} -name "*.blend" | head -1))

if [ -z "${project_file}" ]; then
    echo "ERR: unable to find blender project file in ${PROJECT_DIR}"
    exit 1
fi

# render
echo "rendering frame ${RENDER_FRAME}"
cd $project_dir
cmd="/opt/blender/blender -b $project_file -P ${WORK_DIR}/render.py -f ${RENDER_FRAME}"
echo "executing: $cmd"
$cmd

if [ ! -z "${S3_ENDPOINT}" ]; then
    echo "uploading to s3"
    env | grep -v SECRET
    export MC_HOST_render="https://${S3_ACCESS_KEY_ID}:${S3_ACCESS_KEY_SECRET}@${S3_ENDPOINT}"
    timeout 10s mc mirror --overwrite ${OUTPUT_DIR}/ render/${S3_OUTPUT_DIR}/${PROJECT_ID}/
fi
