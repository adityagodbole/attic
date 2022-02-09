cmake_minimum_required(VERSION 3.21)
project(cpp)
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_BUILD_TYPE Debug)
add_compile_options(-Wall -Wextra -Wno-unused-function -Wno-unused-variable)

set(deps
        nlohmann_json/3.10.5)

include(auto.cmake)
use_gtest()
autobuild(${deps})
